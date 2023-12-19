package chain

import (
	"context"
	sdkmath "cosmossdk.io/math"
	"fmt"
	"github.com/cometbft/cometbft/rpc/client"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/skip-mev/petri/provider"
	"github.com/skip-mev/petri/wallet"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"math"
	"strings"
	"time"
)

type Chain struct {
	Config ChainConfig

	Validators       []*Node
	ValidatorWallets []*wallet.CosmosWallet

	Nodes []*Node
}

type ChainConfig struct {
	Denom         string
	Decimals      uint64
	NumValidators int
	NumNodes      int

	BinaryName string

	Image        provider.ImageDefinition
	SidecarImage provider.ImageDefinition

	GasPrices     string
	GasAdjustment float64

	Bech32Prefix string

	EncodingConfig testutil.TestEncodingConfig

	HomeDir        string
	SidecarHomeDir string
	SidecarPorts   []string
	SidecarArgs    []string

	CoinType string
	ChainId  string

	ModifyGenesis func([]byte) ([]byte, error)
}

func CreateChain(ctx context.Context, infraProvider provider.Provider, config ChainConfig) (*Chain, error) {
	var chain Chain

	validators := make([]*Node, 0)

	for i := 0; i < config.NumValidators; i++ {
		def := provider.TaskDefinition{
			Name:          fmt.Sprintf("%s-validator-%d", config.ChainId, i),
			ContainerName: fmt.Sprintf("%s-validator-%d", config.ChainId, i),
			Image:         config.Image,
			Ports:         []string{"9090", "26656", "26657", "80"},
			Sidecars: []provider.TaskDefinition{
				{
					Name:          fmt.Sprintf("%s-sidecar-%d", config.ChainId, i),
					ContainerName: fmt.Sprintf("%s-sidecar-%d", config.ChainId, i),
					Image:         config.SidecarImage,
					DataDir:       "/oracle",
					Ports:         []string{"80", "8081", "8080"},
					Entrypoint: []string{
						"oracle",
						"--oracle-config-path", "/oracle/oracle.toml",
						"--metrics-config-path", "/oracle/metrics.toml",
						"--host", "0.0.0.0",
						"--port", "8080",
					},
				},
			},
			Command: []string{"--home", config.HomeDir},
			DataDir: config.HomeDir,
		}
		task, err := provider.CreateTask(ctx, infraProvider, def)

		if err != nil {
			return nil, err
		}

		validator := &Node{
			isValidator: true,
			chain:       &chain,
			Task:        task,
		}

		validators = append(validators, validator)
	}

	chain.Validators = validators
	chain.ValidatorWallets = make([]*wallet.CosmosWallet, len(validators))

	nodes := make([]*Node, 0)

	for i := 0; i < config.NumNodes; i++ {
		def := provider.TaskDefinition{
			Name:          fmt.Sprintf("%s-node-%d", config.ChainId, i),
			ContainerName: fmt.Sprintf("%s-node-%d", config.ChainId, i),
			Image:         config.Image,
			DataDir:       config.HomeDir,
			Command:       []string{"--home", config.HomeDir},
			Ports:         []string{"9090", "26656", "26657", "80"},
			Sidecars: []provider.TaskDefinition{
				{
					Name:          fmt.Sprintf("%s-node-%d-sidecar-0", config.ChainId, i),
					ContainerName: fmt.Sprintf("%s-node-%d-sidecar-0", config.ChainId, i),
					Image:         config.SidecarImage,
					DataDir:       "/oracle",
					Ports:         []string{"80", "8081", "8080"},
					Entrypoint: []string{
						"oracle",
						"--oracle-config-path", "/oracle/oracle.toml",
						"--metrics-config-path", "/oracle/metrics.toml",
						"--host", "0.0.0.0",
						"--port", "8080",
					},
				},
			},
		}
		task, err := provider.CreateTask(ctx, infraProvider, def)

		if err != nil {
			return nil, err
		}

		node := &Node{
			isValidator: false,
			chain:       &chain,
			Task:        task,
		}

		nodes = append(nodes, node)
	}

	chain.Nodes = nodes

	chain.Config = config

	return &chain, nil
}

func (c *Chain) Height(ctx context.Context) (uint64, error) {
	node := c.GetFullNode()

	client, err := node.GetTMClient(ctx)

	if err != nil {
		return 0, err
	}

	block, err := client.Block(context.Background(), nil)

	if err != nil {
		return 0, err
	}

	return uint64(block.Block.Height), nil
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
		eg.Go(func() error {
			if err := v.InitHome(ctx); err != nil {
				return err
			}

			validatorWallet, err := v.CreateWallet(ctx, validatorKey)

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

			if err := v.GenerateGentx(ctx, genesisSelfDelegation); err != nil {
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

	faucetWallet, err := c.BuildWallet(ctx, FaucetAccountKeyName, "")

	firstValidator := c.Validators[0]

	if err := firstValidator.AddGenesisAccount(ctx, faucetWallet.FormattedAddress(), genesisAmounts); err != nil {
		return err
	}

	for i := 1; i < len(c.Validators); i++ {
		validatorN := c.Validators[i]
		bech32, err := validatorN.KeyBech32(ctx, validatorKey, "acc")

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

	if err := firstValidator.CollectGentxs(ctx); err != nil {
		return err
	}

	genbz, err := firstValidator.GenesisFileContent(ctx)

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
		if err := v.Task.Start(ctx, true); err != nil {
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
		if err := n.Task.Start(ctx, true); err != nil {
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

func (c *Chain) GetFullNode() *Node {
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
