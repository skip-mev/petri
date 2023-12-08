package chain

import (
	"context"
	"encoding/json"
	"fmt"
	tmjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/p2p"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	libclient "github.com/cometbft/cometbft/rpc/jsonrpc/client"
	client "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	authTx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/skip-mev/petri/provider"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	"path"
	"strings"
	"time"
)

const (
	validatorKey = "validator"
)

type Node struct {
	*provider.Task

	isValidator bool
	chain       *Chain
}

func (n *Node) GetTMClient(ctx context.Context) (rpcclient.Client, error) {
	addr, err := n.Task.GetExternalAddress(ctx, "26657/tcp")

	if err != nil {
		panic(err)
	}

	httpAddr := fmt.Sprintf("http://%s", addr)

	httpClient, err := libclient.DefaultHTTPClient(httpAddr)
	if err != nil {
		return nil, err
	}

	httpClient.Timeout = 10 * time.Second
	rpcClient, err := rpchttp.NewWithClient(httpAddr, "/websocket", httpClient)
	if err != nil {
		return nil, err
	}

	return rpcClient, nil
}

func (n *Node) GetGRPCClient(ctx context.Context) (*grpc.ClientConn, error) {
	grpcAddr, err := n.GetExternalAddress(ctx, "9090/tcp")
	if err != nil {
		return nil, err
	}

	// create the client
	cc, err := grpc.Dial(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return cc, nil
}

func (n *Node) Height(ctx context.Context) (uint64, error) {
	client, err := n.GetTMClient(ctx)

	if err != nil {
		return 0, err
	}

	block, err := client.Block(ctx, nil)

	if err != nil {
		return 0, err
	}

	return uint64(block.Block.Height), nil
}

func (n *Node) NodeId(ctx context.Context) (string, error) {
	// This used to call p2p.LoadNodeKey against the file on the host,
	// but because we are transitioning to operating on Docker volumes,
	// we only have to tmjson.Unmarshal the raw content.
	j, err := n.Task.ReadFile(ctx, "config/node_key.json")
	if err != nil {
		return "", fmt.Errorf("getting node_key.json content: %w", err)
	}

	var nk p2p.NodeKey
	if err := tmjson.Unmarshal(j, &nk); err != nil {
		return "", fmt.Errorf("unmarshaling node_key.json: %w", err)
	}

	return string(nk.ID()), nil
}

func (n *Node) BinCommand(command ...string) []string {
	command = append([]string{n.chain.Config.BinaryName}, command...)
	return append(command,
		"--home", n.chain.Config.HomeDir,
	)
}

func (n *Node) NodeCommand(command ...string) ([]string, error) {
	command = n.BinCommand(command...)
	ip, err := n.GetIP(context.Background())

	if err != nil {
		return []string{}, nil
	}

	return append(command,
		"--node", fmt.Sprintf("tcp://%s:26657", ip),
	), nil
}

func (n *Node) TxCommand(keyName string, command []string) ([]string, error) {
	command = append([]string{"tx"}, command...)
	var gasPriceFound, gasAdjustmentFound, feesFound = false, false, false
	for i := 0; i < len(command); i++ {
		if command[i] == "--gas-prices" {
			gasPriceFound = true
		}
		if command[i] == "--gas-adjustment" {
			gasAdjustmentFound = true
		}
		if command[i] == "--fees" {
			feesFound = true
		}
	}
	if !gasPriceFound && !feesFound {
		command = append(command, "--gas-prices", n.chain.Config.GasPrices)
	}
	if !gasAdjustmentFound {
		command = append(command, "--gas-adjustment", fmt.Sprint(n.chain.Config.GasAdjustment))
	}
	return n.BinCommand(append(command,
		"--from", keyName,
		"--keyring-backend", keyring.BackendTest,
		"--output", "json",
		"-y",
		"--chain-id", n.chain.Config.ChainId,
	)...), nil
}

func (n *Node) ExecTx(ctx context.Context, keyName string, command []string) (string, error) {
	txCmd, err := n.TxCommand(keyName, command)

	if err != nil {
		return "", fmt.Errorf("failed to get tx command: %v", err)
	}

	stdout, _, err := n.RunCommand(ctx, txCmd)

	cleanedStdout := cleanUpStdoutLines(stdout)

	if err != nil {
		return "", fmt.Errorf("failed to run tx command: %v", err)
	}

	output := CosmosTx{}

	for _, line := range strings.Split(cleanedStdout, "\n") {
		err = json.Unmarshal([]byte(line), &output)
		if err == nil {
			break
		}
	}

	if err != nil {
		return "", err
	}
	if output.Code != 0 {
		return output.TxHash, fmt.Errorf("transaction failed with code %d: %s", output.Code, output.RawLog)
	}
	if err := n.chain.WaitForBlocks(ctx, 2); err != nil {
		return "", err
	}
	return output.TxHash, nil
}

func (n *Node) SetPersistentPeers(ctx context.Context, peers string) error {
	// read the consensus config from the node
	bz, err := n.ReadFile(ctx, "config/config.toml")
	if err != nil {
		panic(err)
	}

	var config map[string]interface{}
	err = toml.Unmarshal(bz, &config)
	if err != nil {
		panic(err)
	}

	p2pConfig, ok := config["p2p"].(map[string]interface{})
	if !ok {
		panic("oracle config not found")
	}

	p2pConfig["persistent_peers"] = peers

	config["p2p"] = p2pConfig
	bz, err = toml.Marshal(config)
	if err != nil {
		panic(err)
	}

	// Write back the consensus config.
	err = n.WriteFile(ctx, "config/config.toml", bz)
	if err != nil {
		panic(err)
	}

	return nil
}

func (n *Node) SendFunds(ctx context.Context, keyName string, amount WalletAmount) error {
	_, err := n.ExecTx(ctx,
		keyName, []string{"bank", "send", keyName,
			amount.Address, fmt.Sprintf("%s%s", amount.Amount.String(), amount.Denom)},
	)

	return err
}

func (n *Node) VoteOnProposal(ctx context.Context, keyName, proposalID, vote string) error {
	_, err := n.ExecTx(ctx, keyName,
		[]string{"gov", "vote",
			proposalID, vote, "--gas", "auto"},
	)
	return err
}

func (n *Node) SubmitProposal(ctx context.Context, keyName string, prop TxProposalv1) (string, error) {
	// Write msg to container
	file := "proposal.json"
	propJson, err := json.MarshalIndent(prop, "", " ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal gov v1 proposal: %v", err)
	}

	if err := n.WriteFile(ctx, file, propJson); err != nil {
		return "", fmt.Errorf("writing contract file to docker volume: %w", err)
	}

	command := []string{
		"gov", "submit-proposal",
		path.Join(n.chain.Config.HomeDir, file), "--gas", "auto",
	}

	return n.ExecTx(ctx, keyName, command)
}

func (n *Node) GetTransaction(clientCtx client.Context, txHash string) (*types.TxResponse, error) {
	// Retry because sometimes the tx is not committed to state yet.
	var txResp *types.TxResponse
	err := retry.Do(func() error {
		var err error
		txResp, err = authTx.QueryTx(clientCtx, txHash)
		return err
	},
		// retry for total of 3 seconds
		retry.Attempts(15),
		retry.Delay(200*time.Millisecond),
		retry.DelayType(retry.FixedDelay),
		retry.LastErrorOnly(true),
	)
	return txResp, err
}

// CliContext creates a new Cosmos SDK client context
func (n *Node) CliContext() (client.Context, error) {
	cfg := n.chain.Config

	tmClient, err := n.GetTMClient(context.Background())

	if err != nil {
		return client.Context{}, err
	}

	return client.Context{
		Client:            tmClient,
		ChainID:           cfg.ChainId,
		InterfaceRegistry: cfg.EncodingConfig.InterfaceRegistry,
		Input:             os.Stdin,
		Output:            os.Stdout,
		OutputFormat:      "json",
		LegacyAmino:       cfg.EncodingConfig.Amino,
		TxConfig:          cfg.EncodingConfig.TxConfig,
	}, nil
}
