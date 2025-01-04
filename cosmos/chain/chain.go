package chain

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	sdkmath "cosmossdk.io/math"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	sdkclient "github.com/cosmos/cosmos-sdk/client"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/skip-mev/petri/core/v2/provider"
	petritypes "github.com/skip-mev/petri/core/v2/types"
)

// Chain is a logical representation of a Cosmos-based blockchain
type Chain struct {
	Config petritypes.ChainConfig

	logger *zap.Logger

	Validators []petritypes.NodeI
	Nodes      []petritypes.NodeI

	FaucetWallet petritypes.WalletI

	ValidatorWallets []petritypes.WalletI

	mu sync.RWMutex
}

var _ petritypes.ChainI = &Chain{}

// CreateChain creates the Chain object and initializes the node tasks, their backing compute and the validator wallets
func CreateChain(ctx context.Context, logger *zap.Logger, infraProvider provider.ProviderI, config petritypes.ChainConfig) (*Chain, error) {
	if err := config.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("failed to validate chain config: %w", err)
	}

	var chain Chain

	chain.mu = sync.RWMutex{}
	chain.Config = config
	chain.logger = logger.Named("chain").With(zap.String("chain_id", config.ChainId))

	chain.logger.Info("creating chain")

	validators := make([]petritypes.NodeI, 0)
	nodes := make([]petritypes.NodeI, 0)

	var eg errgroup.Group

	chain.logger.Info("creating validators and nodes", zap.Int("num_validators", config.NumValidators), zap.Int("num_nodes", config.NumNodes))

	for i := 0; i < config.NumValidators; i++ {
		i := i
		eg.Go(func() error {
			validatorName := fmt.Sprintf("%s-validator-%d", config.ChainId, i)

			logger.Info("creating validator", zap.String("name", validatorName))

			validator, err := config.NodeCreator(ctx, logger, infraProvider, petritypes.NodeConfig{
				Index:       i,
				Name:        validatorName,
				IsValidator: true,
				Chain:       &chain,
			})
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
			nodeName := fmt.Sprintf("%s-node-%d", config.ChainId, i)

			logger.Info("creating node", zap.String("name", nodeName))

			node, err := config.NodeCreator(ctx, logger, infraProvider, petritypes.NodeConfig{
				Index:       i,
				Name:        nodeName,
				IsValidator: true,
				Chain:       &chain,
			})
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
	chain.Config = config
	chain.ValidatorWallets = make([]petritypes.WalletI, config.NumValidators)

	return &chain, nil
}

// GetConfig returns the Chain's configuration
func (c *Chain) GetConfig() petritypes.ChainConfig {
	return c.Config
}

// Height returns the chain's height from the first available full node in the network
func (c *Chain) Height(ctx context.Context) (uint64, error) {
	node := c.GetFullNode()

	client, err := node.GetTMClient(ctx)

	c.logger.Debug("fetching height from", zap.String("node", node.GetDefinition().Name), zap.String("ip", client.Remote()))

	if err != nil {
		return 0, err
	}

	status, err := client.Status(context.Background())
	if err != nil {
		return 0, err
	}

	return uint64(status.SyncInfo.LatestBlockHeight), nil
}

// Init initializes the chain. That consists of generating the genesis transactions, genesis file, wallets,
// the distribution of configuration files and starting the network nodes up
func (c *Chain) Init(ctx context.Context) error {
	decimalPow := int64(math.Pow10(int(c.Config.Decimals)))

	genesisCoin := types.Coin{
		Amount: sdkmath.NewIntFromBigInt(c.GetConfig().GetGenesisBalance()).MulRaw(decimalPow),
		Denom:  c.Config.Denom,
	}
	c.logger.Info("creating genesis accounts", zap.String("coin", genesisCoin.String()))

	genesisSelfDelegation := types.Coin{
		Amount: sdkmath.NewIntFromBigInt(c.GetConfig().GetGenesisDelegation()).MulRaw(decimalPow),
		Denom:  c.Config.Denom,
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

			validatorWallet, err := v.CreateWallet(ctx, petritypes.ValidatorKeyName, c.Config.WalletConfig)
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
	faucetWallet, err := c.BuildWallet(ctx, petritypes.FaucetAccountKeyName, "", c.Config.WalletConfig)
	if err != nil {
		return err
	}

	c.FaucetWallet = faucetWallet

	firstValidator := c.Validators[0]

	if err := firstValidator.AddGenesisAccount(ctx, faucetWallet.FormattedAddress(), genesisAmounts); err != nil {
		return err
	}

	for i := 1; i < len(c.Validators); i++ {
		validatorN := c.Validators[i]
		validatorWalletAddress := c.ValidatorWallets[i].FormattedAddress()
		eg.Go(func() error {
			bech32, err := validatorN.KeyBech32(ctx, petritypes.ValidatorKeyName, "acc")
			if err != nil {
				return err
			}

			c.logger.Info("setting up validator keys", zap.String("validator", validatorN.GetDefinition().Name), zap.String("address", bech32))
			if err := firstValidator.AddGenesisAccount(ctx, bech32, genesisAmounts); err != nil {
				return err
			}

			if err := firstValidator.AddGenesisAccount(ctx, validatorWalletAddress, genesisAmounts); err != nil {
				return err
			}

			if err := validatorN.CopyGenTx(ctx, firstValidator); err != nil {
				return err
			}

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	if err := firstValidator.CollectGenTxs(ctx); err != nil {
		return err
	}

	genbz, err := firstValidator.GenesisFileContent(ctx)
	if err != nil {
		return err
	}

	if c.Config.ModifyGenesis != nil {
		genbz, err = c.Config.ModifyGenesis(genbz)
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
	c.logger.Info("tearing down chain", zap.String("name", c.Config.ChainId))

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

// WaitForBlocks blocks until the chain increases in block height by delta
func (c *Chain) WaitForBlocks(ctx context.Context, delta uint64) error {
	c.logger.Info("waiting for blocks", zap.Uint64("delta", delta))

	start, err := c.Height(ctx)
	if err != nil {
		return err
	}

	cur := start

	for {
		c.logger.Debug("waiting for blocks", zap.Uint64("desired_delta", delta), zap.Uint64("current_delta", cur-start))

		if cur-start >= delta {
			break
		}

		cur, err = c.Height(ctx)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		// We assume the chain will eventually return a non-zero height, otherwise
		// this may block indefinitely.
		if cur == 0 {
			time.Sleep(2 * time.Second)
			continue
		}

		time.Sleep(2 * time.Second)
	}
	return nil
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

// GetTxConfig returns a Cosmos SDK TxConfig
func (c *Chain) GetTxConfig() sdkclient.TxConfig {
	return c.Config.EncodingConfig.TxConfig
}

// GetInterfaceRegistry returns a Cosmos SDK InterfaceRegistry
func (c *Chain) GetInterfaceRegistry() codectypes.InterfaceRegistry {
	return c.Config.EncodingConfig.InterfaceRegistry
}
