package setup

import (
	networkgen "github.com/skip-mev/petri/networkgen"
	"github.com/skip-mev/petri/provider"
)

type SetupHandler func(t networkgen.Testnet)
