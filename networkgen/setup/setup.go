package setup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	networkgen "github.com/skip-mev/petri/networkgen"
	"github.com/skip-mev/petri/networkgen/imported"
)

const (
	PrivvalAddressTCP     = "tcp://0.0.0.0:27559"
	PrivvalAddressUNIX    = "unix:///var/run/privval.sock"
	PrivvalKeyFile        = "config/priv_validator_key.json"
	PrivvalStateFile      = "data/priv_validator_state.json"
	PrivvalDummyKeyFile   = "config/dummy_validator_key.json"
	PrivvalDummyStateFile = "data/dummy_validator_state.json"
)

// SetupHandler is used to write all necessary configs to Testnet.Dir.
type SetupHandler func(t networkgen.Testnet) error

// DefaultSetupHandler is the default setup handler for cometbft networks. This method will:
// - create the testnet directory structure
// - create the genesis in accordance with the GenesisExtension from the Testnet
// - setup node config directories (app.toml, config.toml)
// - create the priv_validator_key.json and priv_validator_state.json files for each node
// - create the node_key.json file for each node
// This method will return an error if any of the above steps fail. This logic is borrowed from here: https://github.com/cometbft/cometbft/blob/main/test/e2e/runner/setup.go
func DefaultSetupHandler(log log.Logger) SetupHandler {
	return func(t networkgen.Testnet) error {
		log.Info("setup", "msg", fmt.Sprintf("Generating testnet files in %q", t.Testnet.Dir))

		// create testnet directory structure
		if err := os.MkdirAll(t.Testnet.Dir, os.ModePerm); err != nil {
			log.Error("setup", "msg", fmt.Sprintf("Failed to create testnet directory: %v", err))
			return err
		}

		// create the genesis
		genesis, err := t.GenesisExtension.GetGenesis(t.Testnet)
		if err != nil {
			log.Error("setup", "msg", fmt.Sprintf("Failed to create genesis: %v", err))
			return err
		}

		// setup node config directories
		for _, node := range t.Nodes {
			nodeDir := filepath.Join(t.Testnet.Dir, node.Name)

			dirs := []string{
				filepath.Join(nodeDir, "config"),
				filepath.Join(nodeDir, "data"),
				filepath.Join(nodeDir, "data", "app"),
			}
			for _, dir := range dirs {
				err := os.MkdirAll(dir, 0o755)
				if err != nil {
					return err
				}
			}

			// create config.toml
			cfg, err := imported.MakeConfig(node)
			if err != nil {
				log.Error("setup", "msg", fmt.Sprintf("Failed to create config: %v", err))
				return err
			}

			// keep invalid txs in cache
			cfg.Mempool.KeepInvalidTxsInCache = true

			// generate app-config
			acc, ok := t.AppConfigExtension[node.Name]
			if !ok {
				log.Error("setup", "msg", fmt.Sprintf("no app config extension for node %s", node.Name))
				return fmt.Errorf("no app config extension for node %s", node.Name)
			}

			appCfg, err := acc.GetAppConfig(node)
			if err != nil {
				return err
			}

			// write configs
			err = os.WriteFile(filepath.Join(nodeDir, "config", "app.toml"), appCfg, 0o644) //nolint:gosec
			if err != nil {
				return err
			}
	
			err = genesis.SaveAs(filepath.Join(nodeDir, "config", "genesis.json"))
			if err != nil {
				return err
			}
			
			// create node keys (p2p-key and privval-key)
			err = (&p2p.NodeKey{PrivKey: node.NodeKey}).SaveAs(filepath.Join(nodeDir, "config", "node_key.json"))
			if err != nil {
				log.Error("setup", "msg", fmt.Sprintf("Failed to create node key: %v", err))
				return err
			}
	
			(privval.NewFilePV(node.PrivvalKey,
				filepath.Join(nodeDir, PrivvalKeyFile),
				filepath.Join(nodeDir, PrivvalStateFile),
			)).Save()
	
			// Set up a dummy validator. CometBFT requires a file PV even when not used, so we
			// give it a dummy such that it will fail if it actually tries to use it.
			(privval.NewFilePV(ed25519.GenPrivKey(),
				filepath.Join(nodeDir, PrivvalDummyKeyFile),
				filepath.Join(nodeDir, PrivvalDummyStateFile),
			)).Save()
		}

		return t.WriteLoadConfig()
	}
}
