package docker

import (
	"github.com/docker/go-connections/nat"
	"github.com/skip-mev/petri/provider"
)

func convertTaskDefinitionPortsToNat(definition provider.TaskDefinition) (nat.PortMap, error) {
	bindings := nat.PortMap{}

	_, bindings, err := nat.ParsePortSpecs(definition.Ports)

	if err != nil {
		return nil, err
	}

	return bindings, nil
}
