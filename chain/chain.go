package chain

import (
	"context"
	sdkmath "cosmossdk.io/math"
	"fmt"
	"github.com/cometbft/cometbft/rpc/client"
	sdkclient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/skip-mev/petri/provider"
	petritypes "github.com/skip-mev/petri/types"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"math"
	"strings"
	"time"
)

type Chain struct {
	Config petritypes.ChainConfig

	Validators []petritypes.NodeI
	Nodes      []petritypes.NodeI

	FaucetWallet petritypes.WalletI

	ValidatorWallets []petritypes.WalletI
}

var _ petritypes.ChainI = &Chain{}

func CreateChain(ctx context.Context, infraProvider provider.Provider, config petritypes.ChainConfig) (*Chain, error) {
	var chain Chain

	chain.Config = config

	validators := make([]petritypes.NodeI, 0)
	nodes := make([]petritypes.NodeI, 0)

	for i := 0; i < config.NumValidators; i++ {
		validator, err := config.NodeCreator(ctx, petritypes.NodeConfig{
			Name:        fmt.Sprintf("%s-validator-%d", config.ChainId, i),
			IsValidator: true,
			Provider:    infraProvider,
			Chain:       &chain,
		})

		if err != nil {
			return nil, err
		}

		validators = append(validators, validator)
	}

	for i := 0; i < config.NumNodes; i++ {
		node, err := config.NodeCreator(ctx, petritypes.NodeConfig{
			Name:        fmt.Sprintf("%s-node-%d", config.ChainId, i),
			IsValidator: true,
			Provider:    infraProvider,
			Chain:       &chain,
		})

		if err != nil {
			return nil, err
		}

		nodes = append(nodes, node)
	}

	chain.Nodes = nodes
	chain.Validators = validators
	chain.Config = config
	chain.ValidatorWallets = make([]petritypes.WalletI, config.NumValidators)

	return &chain, nil
}

func (c *Chain) GetConfig() petritypes.ChainConfig {
	return c.Config
}

func (c *Chain) Height(ctx context.Context) (uint64, error) {
	node := c.GetFullNode()

	client, err := node.GetTMClient(ctx)

	if err != nil {
		return 0, err
	}

	block, err := client.Status(context.Background())

	if err != nil {
		return 0, err
	}

	return uint64(status.SyncInfo.LatestBlockHeight), nil
}

func (c *Chain) Init(ctx context.Context) error {
	decimalPow := int64(math.Pow10(int(c.Config.Decimals)))

	genesisCoin := types.Coin{
		Amount: sdkmath.NewInt(10_000_000).MulRaw(decimalPow),
		Denom:  c.Config.Denom,
	}

	genesisSelfDelegation := types.Coin{
		Amount: sdkmath.NewInt(5_000_000).MulRaw(decimalPow),
		Denom:  c.Config.Denom,
	}

	genesisAmounts := []types.Coin{genesisCoin}

	eg := new(errgroup.Group)

	for idx, v := range c.Validators {
		v := v
		idx := idx
		eg.Go(func() error {
			if err := v.InitHome(ctx); err != nil {
				return err
			}

			validatorWallet, err := v.CreateWallet(ctx, petritypes.ValidatorKeyName)

			if err != nil {
				return err
			}

			c.ValidatorWallets[idx] = validatorWallet

			bech32 := validatorWallet.FormattedAddress()

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
			if err := n.InitHome(ctx); err != nil {
				return err
			}

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	faucetWallet, err := c.BuildWallet(ctx, petritypes.FaucetAccountKeyName, "")

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
		bech32, err := validatorN.KeyBech32(ctx, petritypes.ValidatorKeyName, "acc")

		if err != nil {
			return err
		}

		if err := firstValidator.AddGenesisAccount(ctx, bech32, genesisAmounts); err != nil {
			return err
		}

		if err := validatorN.CopyGenTx(ctx, firstValidator); err != nil {
			return err
		}
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

	if err != nil {
		return err
	}

	for _, v := range c.Validators {
		if err := v.OverwriteGenesisFile(ctx, genbz); err != nil {
			return err
		}
		if err := v.SetDefaultConfigs(ctx); err != nil {
			return err
		}
		if err := v.SetPersistentPeers(ctx, peers); err != nil {
			return err
		}
	}

	for _, v := range c.Validators {
		if err := v.GetTask().Start(ctx, true); err != nil {
			return err
		}
	}

	for _, n := range c.Nodes {
		if err := n.OverwriteGenesisFile(ctx, genbz); err != nil {
			return err
		}
		if err := n.SetDefaultConfigs(ctx); err != nil {
			return err
		}
		if err := n.SetPersistentPeers(ctx, peers); err != nil {
			return err
		}
	}

	for _, n := range c.Nodes {
		if err := n.GetTask().Start(ctx, true); err != nil {
			return err
		}
	}

	return nil
}

func (c *Chain) Teardown(ctx context.Context) error {
	for _, v := range c.Validators {
		if err := v.GetTask().Destroy(ctx, true); err != nil {
			return err
		}
	}

	for _, n := range c.Nodes {
		if err := n.GetTask().Destroy(ctx, true); err != nil {
			return err
		}
	}

	return nil
}

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

func (c *Chain) GetGRPCClient(ctx context.Context) (*grpc.ClientConn, error) {
	return c.GetFullNode().GetGRPCClient(ctx)
}

func (c *Chain) GetTMClient(ctx context.Context) (client.Client, error) {
	return c.GetFullNode().GetTMClient(ctx)
}

func (c *Chain) GetFullNode() petritypes.NodeI {
	if len(c.Nodes) > 0 {
		// use first full node
		return c.Nodes[0]
	}
	// use first validator
	return c.Validators[0]
}

func (c *Chain) WaitForBlocks(ctx context.Context, delta uint64) error {
	start, err := c.Height(ctx)

	if err != nil {
		return err
	}

	cur := start

	for {
		if cur-start >= delta {
			break
		}

		cur, err = c.Height(ctx)
		if err != nil {
			continue
		}
		// We assume the chain will eventually return a non-zero height, otherwise
		// this may block indefinitely.
		if cur == 0 {
			continue
		}

		time.Sleep(2 * time.Second)
	}
	return nil
}

func (c *Chain) WaitForHeight(ctx context.Context, desiredHeight uint64) error {
	height, err := c.Height(ctx)

	if err != nil {
		return err
	}

	for {
		if height >= desiredHeight {
			break
		}

		height, err = c.Height(ctx)
		if err != nil {
			continue
		}

		// We assume the chain will eventually return a non-zero height, otherwise
		// this may block indefinitely.
		if height == 0 {
			continue
		}

		time.Sleep(2 * time.Second)
	}

	return nil
}

func (c *Chain) GetValidators() []petritypes.NodeI {
	return c.Validators
}

func (c *Chain) GetNodes() []petritypes.NodeI {
	return c.Nodes
}

func (c *Chain) GetValidatorWallets() []petritypes.WalletI {
	return c.ValidatorWallets
}

func (c *Chain) GetFaucetWallet() petritypes.WalletI {
	return c.FaucetWallet
}

func (c *Chain) GetTxConfig() sdkclient.TxConfig {
	return c.Config.EncodingConfig.TxConfig
}
