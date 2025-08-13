package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/skip-mev/petri/cosmos/v3/wallet"

	sdkmath "cosmossdk.io/math"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/skip-mev/petri/core/v3/provider"
	petritypes "github.com/skip-mev/petri/core/v3/types"
)

type PackagedState struct {
	State
	ValidatorStates  [][]byte
	NodeStates       [][]byte
	ValidatorWallets []string
	FaucetWallet     string
}

type State struct {
	Config petritypes.ChainConfig
}

// Chain is a logical representation of a Cosmos-based blockchain
type Chain struct {
	State State

	logger *zap.Logger

	Validators []petritypes.NodeI
	Nodes      []petritypes.NodeI

	FaucetWallet petritypes.WalletI

	ValidatorWallets []petritypes.WalletI

	mu sync.RWMutex

	// useExternalAddresses determines whether to use external addresses (DigitalOcean)
	// or internal addresses (Docker) for peer strings
	useExternalAddresses bool
}

var _ petritypes.ChainI = &Chain{}

// CreateChain creates the Chain object and initializes the node tasks, their backing compute and the validator wallets
func CreateChain(ctx context.Context, logger *zap.Logger, infraProvider provider.ProviderI, config petritypes.ChainConfig, opts petritypes.ChainOptions) (*Chain, error) {
	providerType := infraProvider.GetType()
	if err := config.ValidateBasic(providerType); err != nil {
		return nil, fmt.Errorf("failed to validate chain config: %w", err)
	}

	if err := opts.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("failed to validate chain options: %w", err)
	}

	var chain Chain

	chain.mu = sync.RWMutex{}
	chain.State = State{
		Config: config,
	}
	chain.useExternalAddresses = providerType == petritypes.DigitalOcean

	chain.logger = logger.Named("chain").With(zap.String("chain_id", config.ChainId))
	chain.logger.Info("creating chain")

	validators, nodes, err := createNodes(ctx, logger, &chain, infraProvider, providerType, config, opts)
	if err != nil {
		return nil, err
	}
	logger.Info("created nodes", zap.Int("validators", len(validators)), zap.Int("nodes", len(nodes)))

	chain.Nodes = nodes
	chain.Validators = validators
	chain.ValidatorWallets = make([]petritypes.WalletI, len(validators))

	return &chain, nil
}

func createNodes(ctx context.Context, logger *zap.Logger, chain *Chain, infraProvider provider.ProviderI, providerType string,
	config petritypes.ChainConfig, opts petritypes.ChainOptions) ([]petritypes.NodeI, []petritypes.NodeI, error) {
	if providerType == petritypes.DigitalOcean {
		return createRegionalNodes(ctx, logger, chain, infraProvider, config, opts)
	}

	return createLocalNodes(ctx, logger, chain, infraProvider, config, opts)
}

func createRegionalNodes(ctx context.Context, logger *zap.Logger, chain *Chain, infraProvider provider.ProviderI,
	config petritypes.ChainConfig, opts petritypes.ChainOptions) ([]petritypes.NodeI, []petritypes.NodeI, error) {
	var eg errgroup.Group
	validators := make([]petritypes.NodeI, 0)
	nodes := make([]petritypes.NodeI, 0)

	validatorIndex := 0
	nodeIndex := 0

	for _, regionConfig := range config.RegionConfig {
		region := regionConfig

		for i := 0; i < region.NumValidators; i++ {
			currentValidatorIndex := validatorIndex
			currentRegion := region.Name
			eg.Go(func() error {
				validator, err := createNode(ctx, logger, infraProvider, config, opts,
					currentValidatorIndex, currentRegion, "validator",
					createRegionalNodeOptions(opts.NodeOptions, region))
				if err != nil {
					return err
				}

				chain.mu.Lock()
				validators = append(validators, validator)
				chain.mu.Unlock()
				return nil
			})
			validatorIndex++
		}

		for i := 0; i < region.NumNodes; i++ {
			currentNodeIndex := nodeIndex
			currentRegion := region.Name
			eg.Go(func() error {
				node, err := createNode(ctx, logger, infraProvider, config, opts,
					currentNodeIndex, currentRegion, "node",
					createRegionalNodeOptions(opts.NodeOptions, region))
				if err != nil {
					return err
				}

				chain.mu.Lock()
				nodes = append(nodes, node)
				chain.mu.Unlock()
				return nil
			})
			nodeIndex++
		}
	}

	if err := eg.Wait(); err != nil {
		logger.Error("error creating regional nodes", zap.Error(err))
		return nil, nil, err
	}

	return validators, nodes, nil
}

func createLocalNodes(ctx context.Context, logger *zap.Logger, chain *Chain, infraProvider provider.ProviderI,
	config petritypes.ChainConfig, opts petritypes.ChainOptions) ([]petritypes.NodeI, []petritypes.NodeI, error) {
	var eg errgroup.Group
	validators := make([]petritypes.NodeI, 0)
	nodes := make([]petritypes.NodeI, 0)

	for i := 0; i < config.NumValidators; i++ {
		currentValidatorIndex := i
		eg.Go(func() error {
			validator, err := createNode(ctx, logger, infraProvider, config, opts,
				currentValidatorIndex, "", "validator", opts.NodeOptions)
			if err != nil {
				return err
			}

			chain.mu.Lock()
			validators = append(validators, validator)
			chain.mu.Unlock()
			return nil
		})
	}

	for i := 0; i < config.NumNodes; i++ {
		currentNodeIndex := i
		eg.Go(func() error {
			node, err := createNode(ctx, logger, infraProvider, config, opts,
				currentNodeIndex, "", "node", opts.NodeOptions)
			if err != nil {
				return err
			}

			chain.mu.Lock()
			nodes = append(nodes, node)
			chain.mu.Unlock()
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		logger.Error("error creating local nodes", zap.Error(err))
		return nil, nil, err
	}

	return validators, nodes, nil
}

func createNode(ctx context.Context, logger *zap.Logger, infraProvider provider.ProviderI, config petritypes.ChainConfig,
	opts petritypes.ChainOptions, index int, region string, nodeType string, nodeOptions petritypes.NodeOptions) (petritypes.NodeI, error) {

	var nodeName string
	if region != "" {
		nodeName = fmt.Sprintf("%s-%s-%d-%s", config.Name, nodeType, index, region)
	} else {
		nodeName = fmt.Sprintf("%s-%s-%d", config.Name, nodeType, index)
	}

	logger.Info(fmt.Sprintf("creating %s", nodeType), zap.String("name", nodeName))
	return opts.NodeCreator(ctx, logger, infraProvider, petritypes.NodeConfig{
		Index:       index,
		Name:        nodeName,
		ChainConfig: config,
	}, nodeOptions)
}

func createRegionalNodeOptions(baseOpts petritypes.NodeOptions, region petritypes.RegionConfig) petritypes.NodeOptions {
	applyRegionConfig := func(definition provider.TaskDefinition) provider.TaskDefinition {
		if definition.ProviderSpecificConfig == nil {
			definition.ProviderSpecificConfig = make(map[string]string)
		}
		definition.ProviderSpecificConfig["region"] = region.Name
		definition.ProviderSpecificConfig["size"] = "s-4vcpu-8gb"
		definition.ProviderSpecificConfig["image_id"] = "195881161"
		return definition
	}

	if baseOpts.NodeDefinitionModifier == nil {
		baseOpts.NodeDefinitionModifier = func(definition provider.TaskDefinition, nodeConfig petritypes.NodeConfig) provider.TaskDefinition {
			return applyRegionConfig(definition)
		}
	} else {
		originalModifier := baseOpts.NodeDefinitionModifier
		baseOpts.NodeDefinitionModifier = func(definition provider.TaskDefinition, nodeConfig petritypes.NodeConfig) provider.TaskDefinition {
			definition = originalModifier(definition, nodeConfig)
			return applyRegionConfig(definition)
		}
	}
	return baseOpts
}

// RestoreChain restores a Chain object from a serialized state
func RestoreChain(ctx context.Context, logger *zap.Logger, infraProvider provider.ProviderI, state []byte,
	nodeRestore petritypes.NodeRestorer, walletConfig petritypes.WalletConfig) (*Chain, error) {
	var packagedState PackagedState

	if err := json.Unmarshal(state, &packagedState); err != nil {
		return nil, err
	}

	chain := Chain{
		State:      packagedState.State,
		logger:     logger,
		Validators: make([]petritypes.NodeI, len(packagedState.ValidatorStates)),
		Nodes:      make([]petritypes.NodeI, len(packagedState.NodeStates)),
	}

	eg := new(errgroup.Group)

	for i, vs := range packagedState.ValidatorStates {
		eg.Go(func() error {
			i := i
			v, err := nodeRestore(ctx, logger, vs, infraProvider)

			if err != nil {
				return err
			}

			chain.Validators[i] = v
			return nil
		})
	}

	for i, ns := range packagedState.NodeStates {
		eg.Go(func() error {
			i := i
			v, err := nodeRestore(ctx, logger, ns, infraProvider)

			if err != nil {
				return err
			}

			chain.Nodes[i] = v
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	chain.ValidatorWallets = make([]petritypes.WalletI, len(packagedState.ValidatorWallets))
	for i, mnemonic := range packagedState.ValidatorWallets {
		w, err := wallet.NewWallet(petritypes.ValidatorKeyName, mnemonic, walletConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to restore validator wallet: %w", err)
		}
		chain.ValidatorWallets[i] = w
	}

	if packagedState.FaucetWallet != "" {
		w, err := wallet.NewWallet(petritypes.FaucetAccountKeyName, packagedState.FaucetWallet, walletConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to restore faucet wallet: %w", err)
		}
		chain.FaucetWallet = w
	}

	return &chain, nil
}

// Height returns the chain's height from the first available full node in the network
func (c *Chain) Height(ctx context.Context) (uint64, error) {
	node := c.GetFullNode()

	client, err := node.GetTMClient(ctx)

	if err != nil {
		return 0, err
	}

	c.logger.Debug("fetching height from", zap.String("node", node.GetDefinition().Name), zap.String("ip", client.Remote()))

	status, err := client.Status(context.Background())
	if err != nil {
		return 0, err
	}

	return uint64(status.SyncInfo.LatestBlockHeight), nil
}

// Init initializes the chain. That consists of generating the genesis transactions, genesis file, wallets,
// the distribution of configuration files and starting the network nodes up
func (c *Chain) Init(ctx context.Context, opts petritypes.ChainOptions) error {
	if err := opts.ValidateBasic(); err != nil {
		return fmt.Errorf("failed to validate chain options: %w", err)
	}

	decimalPow := int64(math.Pow10(int(c.GetConfig().Decimals)))

	genesisCoin := types.Coin{
		Amount: sdkmath.NewIntFromBigInt(c.GetConfig().GetGenesisBalance()).MulRaw(decimalPow),
		Denom:  c.GetConfig().Denom,
	}
	c.logger.Info("creating genesis accounts", zap.String("coin", genesisCoin.String()))

	genesisSelfDelegation := types.Coin{
		Amount: sdkmath.NewIntFromBigInt(c.GetConfig().GetGenesisDelegation()).MulRaw(decimalPow),
		Denom:  c.GetConfig().Denom,
	}
	c.logger.Info("creating genesis self-delegations", zap.String("coin", genesisSelfDelegation.String()))

	genesisAmounts := []types.Coin{genesisCoin}

	eg := new(errgroup.Group)

	for idx, v := range c.Validators {
		v := v
		idx := idx
		eg.Go(func() error {
			c.logger.Info("setting up validator home dir", zap.String("validator", v.GetDefinition().Name))
			if err := v.InitHome(ctx); err != nil {
				return fmt.Errorf("error initializing home dir: %v", err)
			}

			validatorWallet, err := v.CreateWallet(ctx, petritypes.ValidatorKeyName, opts.WalletConfig)
			if err != nil {
				return err
			}

			c.ValidatorWallets[idx] = validatorWallet

			bech32, err := v.KeyBech32(ctx, petritypes.ValidatorKeyName, "acc")
			if err != nil {
				return err
			}

			if err := v.AddGenesisAccount(ctx, bech32, genesisAmounts); err != nil {
				return err
			}

			if err := v.GenerateGenTx(ctx, genesisSelfDelegation); err != nil {
				return err
			}

			return nil
		})
	}

	for _, n := range c.Nodes {
		n := n

		eg.Go(func() error {
			c.logger.Info("setting up node home dir", zap.String("node", n.GetDefinition().Name))
			if err := n.InitHome(ctx); err != nil {
				return err
			}

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	c.logger.Info("adding faucet genesis")
	faucetWallet, err := c.BuildWallet(ctx, petritypes.FaucetAccountKeyName, "", opts.WalletConfig)
	if err != nil {
		return err
	}

	c.FaucetWallet = faucetWallet

	firstValidator := c.Validators[0]

	c.logger.Info("first validator name", zap.String("validator", firstValidator.GetDefinition().Name))

	if err := firstValidator.AddGenesisAccount(ctx, faucetWallet.FormattedAddress(), genesisAmounts); err != nil {
		return err
	}

	for i := 1; i < len(c.Validators); i++ {
		validatorN := c.Validators[i]

		bech32, err := validatorN.KeyBech32(ctx, petritypes.ValidatorKeyName, "acc")
		if err != nil {
			return err
		}

		c.logger.Info("setting up validator keys", zap.String("validator", validatorN.GetDefinition().Name), zap.String("address", bech32))
		if err := firstValidator.AddGenesisAccount(ctx, bech32, genesisAmounts); err != nil {
			return fmt.Errorf("failed to add validator %s genesis account: %w", validatorN.GetDefinition().Name, err)
		}

		if err := validatorN.CopyGenTx(ctx, firstValidator); err != nil {
			return fmt.Errorf("failed to copy gentx from %s: %w", validatorN.GetDefinition().Name, err)
		}
	}

	if err := firstValidator.CollectGenTxs(ctx); err != nil {
		return err
	}

	genbz, err := firstValidator.GenesisFileContent(ctx)
	if err != nil {
		return err
	}

	if opts.ModifyGenesis != nil {
		c.logger.Info("modifying genesis")
		genbz, err = opts.ModifyGenesis(genbz)
		if err != nil {
			return err
		}
	}

	chainConfig := c.GetConfig()
	persistentPeers, seeds := "", ""
	var seedNode petritypes.NodeI

	if chainConfig.SetSeedNode {
		if len(c.Nodes) > 0 {
			seedNode = c.Nodes[0]
		} else {
			return fmt.Errorf("no nodes available to be used as seed ")
		}

		if seedNode != nil {
			seeds, err = PeerStrings(ctx, []petritypes.NodeI{seedNode}, c.useExternalAddresses)
			if err != nil {
				return err
			}
		}
	}

	if chainConfig.SetPersistentPeers {
		persistentPeers, err = PeerStrings(ctx, append(c.Nodes, c.Validators...), c.useExternalAddresses)
		if err != nil {
			return err
		}
	}

	for i := range c.Validators {
		v := c.Validators[i]
		eg.Go(func() error {
			c.logger.Info("overwriting genesis for validator", zap.String("validator", v.GetDefinition().Name))
			return configureNode(ctx, v, chainConfig, genbz, persistentPeers, seeds, c.useExternalAddresses, c.logger)
		})
	}

	for i := range c.Nodes {
		n := c.Nodes[i]
		eg.Go(func() error {
			c.logger.Info("overwriting node genesis", zap.String("node", n.GetDefinition().Name))
			return configureNode(ctx, n, chainConfig, genbz, persistentPeers, seeds, c.useExternalAddresses, c.logger)
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	if chainConfig.SetSeedNode && seedNode != nil {
		c.logger.Info("configuring seed node mode", zap.String("seed_node", seedNode.GetDefinition().Name))
		if err := seedNode.SetSeedMode(ctx); err != nil {
			return fmt.Errorf("failed to set seed mode on %s: %w", seedNode.GetDefinition().Name, err)
		}
	}

	for i := range c.Validators {
		v := c.Validators[i]
		eg.Go(func() error {
			c.logger.Info("starting validator task", zap.String("validator", v.GetDefinition().Name))
			if err := v.Start(ctx); err != nil {
				return err
			}
			return nil
		})
	}

	for i := range c.Nodes {
		n := c.Nodes[i]
		eg.Go(func() error {
			c.logger.Info("starting node task", zap.String("node", n.GetDefinition().Name))
			if err := n.Start(ctx); err != nil {
				return err
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

// Teardown destroys all resources related to a chain and its' nodes
func (c *Chain) Teardown(ctx context.Context) error {
	c.logger.Info("tearing down chain", zap.String("name", c.GetConfig().ChainId))

	for _, v := range c.Validators {
		if err := v.Destroy(ctx); err != nil {
			return err
		}
	}

	for _, n := range c.Nodes {
		if err := n.Destroy(ctx); err != nil {
			return err
		}
	}

	return nil
}

// PeerStrings returns a comma-delimited string with the addresses of chain nodes in
// the format of nodeid@host:port. If useExternal is true, it uses external addresses
// (for DigitalOcean), otherwise it uses internal addresses (for Docker).
func PeerStrings(ctx context.Context, peers []petritypes.NodeI, useExternal bool) (string, error) {
	if useExternal {
		return PeerStringsExternal(ctx, peers)
	}
	return PeerStringsInternal(ctx, peers)
}

// PeerStringsInternal returns a comma-delimited string with the addresses of chain nodes in
// the format of nodeid@host:port using the internal private address of the node (used for Docker networks)
func PeerStringsInternal(ctx context.Context, peers []petritypes.NodeI) (string, error) {
	peerStrings := []string{}

	for _, n := range peers {
		ip, err := n.GetIP(ctx)
		if err != nil {
			return "", err
		}

		nodeId, err := n.NodeId(ctx)
		if err != nil {
			return "", err
		}

		peerStrings = append(peerStrings, fmt.Sprintf("%s@%s:26656", nodeId, ip))
	}

	return strings.Join(peerStrings, ","), nil
}

// PeerStringsExternal returns a comma-delimited string with the addresses of chain nodes in
// the format of nodeid@host:port using the external public address of the node (used for public DigitalOcean networks)
func PeerStringsExternal(ctx context.Context, peers []petritypes.NodeI) (string, error) {
	peerStrings := []string{}

	for _, n := range peers {
		externalAddr, err := n.GetExternalAddress(ctx, "26656")
		if err != nil {
			return "", err
		}

		nodeId, err := n.NodeId(ctx)
		if err != nil {
			return "", err
		}

		peerStrings = append(peerStrings, fmt.Sprintf("%s@%s", nodeId, externalAddr))
	}

	return strings.Join(peerStrings, ","), nil
}

// GetGRPCClient returns a gRPC client of the first available node
func (c *Chain) GetGRPCClient(ctx context.Context) (*grpc.ClientConn, error) {
	return c.GetFullNode().GetGRPCClient(ctx)
}

// GetTMClient returns a CometBFT client of the first available node
func (c *Chain) GetTMClient(ctx context.Context) (*rpchttp.HTTP, error) {
	return c.GetFullNode().GetTMClient(ctx)
}

// GetFullNode returns the first available full node in the chain
func (c *Chain) GetFullNode() petritypes.NodeI {
	if len(c.Nodes) > 0 {
		// use random full node
		return c.Nodes[rand.Intn(len(c.Nodes))]
	}
	// use random validator
	return c.Validators[rand.Intn(len(c.Validators))]
}

func (c *Chain) WaitForStartup(ctx context.Context) error {
	ticker := time.NewTicker(1 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			err := c.WaitForHeight(ctx, 1)
			if err != nil {
				c.logger.Error("error waiting for height", zap.Error(err))
				continue
			}
			ticker.Stop()
			return nil
		}
	}
}

// WaitForBlocks blocks until the chain increases in block height by delta
func (c *Chain) WaitForBlocks(ctx context.Context, delta uint64) error {
	c.logger.Info("waiting for blocks", zap.Uint64("delta", delta))

	start, err := c.Height(ctx)
	if err != nil {
		return err
	}

	return c.WaitForHeight(ctx, start+delta)
}

// WaitForHeight blocks until the chain reaches block height desiredHeight
func (c *Chain) WaitForHeight(ctx context.Context, desiredHeight uint64) error {
	c.logger.Info("waiting for height", zap.Uint64("desired_height", desiredHeight))
	for {
		c.logger.Debug("waiting for height", zap.Uint64("desired_height", desiredHeight))

		height, err := c.Height(ctx)
		if err != nil {
			c.logger.Error("failed to get height", zap.Error(err))
			time.Sleep(2 * time.Second)
			continue
		}

		if height >= desiredHeight {
			break
		}

		// We assume the chain will eventually return a non-zero height, otherwise
		// this may block indefinitely.
		if height == 0 {
			time.Sleep(2 * time.Second)
			continue
		}

		time.Sleep(2 * time.Second)
	}

	return nil
}

// GetValidators returns all of the validating nodes in the chain
func (c *Chain) GetValidators() []petritypes.NodeI {
	return c.Validators
}

// GetNodes returns all of the non-validating nodes in the chain
func (c *Chain) GetNodes() []petritypes.NodeI {
	return c.Nodes
}

// GetValidatorWallets returns the wallets that were used to create the Validators on-chain.
// The ordering of the slice should correspond to the ordering of GetValidators
func (c *Chain) GetValidatorWallets() []petritypes.WalletI {
	return c.ValidatorWallets
}

// GetFaucetWallet retunrs a wallet that was funded and can be used to fund other wallets
func (c *Chain) GetFaucetWallet() petritypes.WalletI {
	return c.FaucetWallet
}

// GetConfig is the configuration structure for a logical chain.
func (c *Chain) GetConfig() petritypes.ChainConfig {
	return c.State.Config
}

// Serialize returns the serialized representation of the chain
func (c *Chain) Serialize(ctx context.Context, p provider.ProviderI) ([]byte, error) {
	state := PackagedState{
		State: c.State,
	}

	for _, v := range c.Validators {
		vs, err := v.Serialize(ctx, p)
		if err != nil {
			return nil, err
		}
		state.ValidatorStates = append(state.ValidatorStates, vs)
	}

	for _, n := range c.Nodes {
		ns, err := n.Serialize(ctx, p)
		if err != nil {
			return nil, err
		}

		state.NodeStates = append(state.NodeStates, ns)
	}

	for _, w := range c.ValidatorWallets {
		state.ValidatorWallets = append(state.ValidatorWallets, w.Mnemonic())
	}

	if c.FaucetWallet != nil {
		state.FaucetWallet = c.FaucetWallet.Mnemonic()
	}

	return json.Marshal(state)
}

func configureNode(ctx context.Context, node petritypes.NodeI, chainConfig petritypes.ChainConfig, genbz []byte,
	persistentPeers, seeds string, useExternalAddress bool, logger *zap.Logger) error {
	if err := node.OverwriteGenesisFile(ctx, genbz); err != nil {
		return err
	}

	var p2pExternalAddr string
	var err error
	if useExternalAddress {
		p2pExternalAddr, err = node.GetExternalAddress(ctx, "26656")
		if err != nil {
			return fmt.Errorf("failed to get external address for p2p port: %w", err)
		}
	} else {
		p2pExternalAddr, err = node.GetIP(ctx)
		if err != nil {
			return fmt.Errorf("failed to get ip for p2p port: %w", err)
		}
		p2pExternalAddr = fmt.Sprintf("%s:26656", p2pExternalAddr)
	}

	if err := node.SetChainConfigs(ctx, chainConfig.ChainId, p2pExternalAddr); err != nil {
		return err
	}

	logger.Debug("setting persistent peers", zap.String("persistent_peers", persistentPeers))
	if err := node.SetPersistentPeers(ctx, persistentPeers); err != nil {
		return err
	}

	logger.Debug("setting seeds", zap.String("seeds", seeds))
	if err := node.SetSeedNode(ctx, seeds); err != nil {
		return err
	}

	return nil
}
