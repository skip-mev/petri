package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/skip-mev/petri/cosmos/v3/wallet"
	"math"
	"strings"
	"sync"
	"time"

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
}

var _ petritypes.ChainI = &Chain{}

// CreateChain creates the Chain object and initializes the node tasks, their backing compute and the validator wallets
func CreateChain(ctx context.Context, logger *zap.Logger, infraProvider provider.ProviderI, config petritypes.ChainConfig, opts petritypes.ChainOptions) (*Chain, error) {
	if err := config.ValidateBasic(); err != nil {
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

	chain.logger = logger.Named("chain").With(zap.String("chain_id", config.ChainId))

	chain.logger.Info("creating chain")

	validators := make([]petritypes.NodeI, 0)
	nodes := make([]petritypes.NodeI, 0)

	var eg errgroup.Group

	chain.logger.Info("creating validators and nodes", zap.Int("num_validators", config.NumValidators), zap.Int("num_nodes", config.NumNodes))

	for i := 0; i < config.NumValidators; i++ {
		i := i
		eg.Go(func() error {
			validatorName := fmt.Sprintf("validator-%d", i)

			logger.Info("creating validator", zap.String("name", validatorName))

			validator, err := opts.NodeCreator(ctx, logger, infraProvider, petritypes.NodeConfig{
				Index:       i,
				Name:        validatorName,
				IsValidator: true,
				ChainConfig: config,
			}, opts.NodeOptions)
			if err != nil {
				return err
			}

			chain.mu.Lock()
			validators = append(validators, validator)
			chain.mu.Unlock()

			return nil
		})
	}

	logger.Info("creating nodes")

	for i := 0; i < config.NumNodes; i++ {
		i := i

		eg.Go(func() error {
			nodeName := fmt.Sprintf("node-%d", i)

			logger.Info("creating node", zap.String("name", nodeName))

			node, err := opts.NodeCreator(ctx, logger, infraProvider, petritypes.NodeConfig{
				Index:       i,
				Name:        nodeName,
				IsValidator: true,
				ChainConfig: config,
			}, opts.NodeOptions)
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
		return nil, err
	}

	chain.Nodes = nodes
	chain.Validators = validators
	chain.ValidatorWallets = make([]petritypes.WalletI, config.NumValidators)

	return &chain, nil
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
		w, err := wallet.NewWallet(petritypes.ValidatorKeyName, packagedState.FaucetWallet, walletConfig)
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

	peers, err := c.PeerStrings(ctx)
	if err != nil {
		return err
	}

	for i := range c.Validators {
		v := c.Validators[i]
		eg.Go(func() error {
			c.logger.Info("overwriting genesis for validator", zap.String("validator", v.GetDefinition().Name))
			if err := v.OverwriteGenesisFile(ctx, genbz); err != nil {
				return err
			}
			if err := v.SetDefaultConfigs(ctx); err != nil {
				return err
			}
			if err := v.SetPersistentPeers(ctx, peers); err != nil {
				return err
			}
			return nil
		})
	}

	for i := range c.Nodes {
		n := c.Nodes[i]
		eg.Go(func() error {
			c.logger.Info("overwriting node genesis", zap.String("node", n.GetDefinition().Name))
			if err := n.OverwriteGenesisFile(ctx, genbz); err != nil {
				return err
			}
			if err := n.SetDefaultConfigs(ctx); err != nil {
				return err
			}
			if err := n.SetPersistentPeers(ctx, peers); err != nil {
				return err
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
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
// the format of nodeid@host:port
func (c *Chain) PeerStrings(ctx context.Context) (string, error) {
	peerStrings := []string{}

	for _, n := range append(c.Validators, c.Nodes...) {
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
		// use first full node
		return c.Nodes[0]
	}
	// use first validator
	return c.Validators[0]
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
