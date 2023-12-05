package types

import (
	"fmt"
	"path/filepath"

	"github.com/cometbft/cometbft/test/e2e/pkg"
	"github.com/informalsystems/tm-load-test/pkg/loadtest"
	cmttypes "github.com/cometbft/cometbft/types"
)

const (
	// default load config file name
	DefaultLoadConfigFile = "load_config.toml"
)

type (
	// Manifest is a generic interface for a manifest that determines how the underlying cometbft / cosmos-sdk network should be created.
	// This object should be implemented by chains intending to integrate the networkgen library, as this enables chains to arbitrarily customize
	// their network configurations in accordance with their app's requirements.
	Manifest interface {
		// create the manifest from a file
		FromFile(file string) (Manifest, error)

		File() string

		// return the cometbft manifest from this manifest
		GetManifest() e2e.Manifest

		// get the AppConfigExtensions from this manifest
		GetAppConfigExtensions() map[string]AppConfigExtension

		// get the GenesisExtension from this manifest
		GetGenesisExtension() GenesisExtension

		// get the LoadConfig from this manifest
		GetLoadConfig() LoadConfig
	}

	// AppConfigExtension is an interface representing the schema for any app-specific additions to the base app.toml file.
	AppConfigExtension interface {
		GetAppConfig(*e2e.Node) ([]byte, error)
	}

	// GenesisExtension is an interface representing the schema for any app-specific additions to the base genesis.json file.
	GenesisExtension interface {
		GetGenesis(*e2e.Testnet) (cmttypes.GenesisDoc, error)
	}

	// LoadConfig is an interface representing the schema for an application specific load-generator
	LoadConfig interface {
		WriteConfigFile(file string) error
		FromFile(string) (LoadConfig, error)
		ToClientFactory() (loadtest.ClientFactory, error)
		Name() string
	}

	// Testnet is a struct representing a cometbft / cosmos-sdk testnet, with app-specific extensions.
	Testnet struct {
		*e2e.Testnet

		// app.toml extensions per node
		AppConfigExtension map[string]AppConfigExtension

		// genesis config for the chain
		GenesisExtension GenesisExtension

		// load config for the load runner
		LoadConfig LoadConfig
	}
)

// TestnetFromManifest creates a testnet from a manifest.
func TestnetFromManifest(m Manifest, ifd e2e.InfrastructureData) (t Testnet, err error) {
	// generate the base testnet from a manifest
	if t.Testnet, err = e2e.NewTestnetFromManifest(m.GetManifest(), m.File(), ifd); err != nil {
		return Testnet{}, err
	}

	// get the app-config extensions
	t.AppConfigExtension = m.GetAppConfigExtensions()

	// get the genesis extension
	t.GenesisExtension = m.GetGenesisExtension()

	// get the load config
	t.LoadConfig = m.GetLoadConfig()

	return t, nil
}

// Validate validates the testnet. Specifically, it ensures that all nodes have an associated AppConfigExtension.
// And that the testnet itself is valid.
func (t Testnet) Validate() error {
	// validate the testnet
	if err := t.Testnet.Validate(); err != nil {
		return err
	}

	// ensure an AppConfigExtension exists for all nodes
	for _, node := range t.Nodes {
		if _, ok := t.AppConfigExtension[node.Name]; !ok {
			return fmt.Errorf("no app config extension for node %s", node.Name)
		}
	}

	return nil
}

// WriteLoadConfig writes the load config to the testnet directory.
func (t Testnet) WriteLoadConfig() error {
	return t.LoadConfig.WriteConfigFile(filepath.Join(t.Testnet.Dir, DefaultLoadConfigFile))
}
