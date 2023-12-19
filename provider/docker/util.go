package docker

import (
	"github.com/docker/go-connections/nat"
	"github.com/skip-mev/petri/provider"
)

func convertTaskDefinitionPortsToPortSet(definition provider.TaskDefinition) nat.PortSet {
	bindings := nat.PortSet{}

	for _, port := range definition.Ports {
		bindings[nat.Port(port)] = struct{}{}
	}

	return bindings
}
