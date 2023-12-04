package types

import (
	"fmt"
	"path/filepath"

	"github.com/cometbft/cometbft/test/e2e/pkg"
	"github.com/informalsystems/tm-load-test/pkg/loadtest"
)

const (
	// default load config file name
	DefaultLoadConfigFile = "load_config.toml"
)

type (
	Manifest[ACE AppConfigExtension] interface {
		// create the manifest from a file
		FromFile(file string) (Manifest[ACE], error)

		File() string

		// return the cometbft manifest from this manifest
		GetManifest() e2e.Manifest

		// get the AppConfigExtensions from this manifest
		GetAppConfigExtensions() map[string]ACE

		// get the GenesisExtension from this manifest
		GetGenesisExtension() GenesisExtension

		// get the LoadConfig from this manifest
		GetLoadConfig() LoadConfig
	}

	AppConfigExtension interface {
		IsTestnetExtension() bool
	}

	GenesisExtension interface {
		IsGenesisExtension() bool
	}

	LoadConfig interface {
		WriteConfigFile(file string) error
		FromFile(string) (LoadConfig, error)
		ToClientFactory() (loadtest.ClientFactory, error)
		Name() string
	}

	Testnet[ACE AppConfigExtension] struct {
		*e2e.Testnet

		// app.toml extensions per node
		AppConfigExtension map[string]ACE

		// genesis config for the chain
		GenesisExtension GenesisExtension

		// load config for the load runner
		LoadConfig LoadConfig
	}
)

func TestnetFromManifest[
	ACE AppConfigExtension,
](m Manifest[ACE], ifd e2e.InfrastructureData) (t Testnet[ACE], err error) {
	// generate the base testnet from a manifest
	if t.Testnet, err = e2e.NewTestnetFromManifest(m.GetManifest(), m.File(), ifd); err != nil {
		return Testnet[ACE]{}, err
	}

	// get the app-config extensions
	t.AppConfigExtension = m.GetAppConfigExtensions()

	// get the genesis extension
	t.GenesisExtension = m.GetGenesisExtension()

	// get the load config
	t.LoadConfig = m.GetLoadConfig()

	return t, nil
}

func (t Testnet[ACE]) Validate() error {
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

func (t Testnet[ACE]) WriteLoadConfig() error {
	return t.LoadConfig.WriteConfigFile(filepath.Join(t.Testnet.Dir, DefaultLoadConfigFile))
}
