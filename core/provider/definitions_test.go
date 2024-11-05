package provider_test

import (
	"github.com/skip-mev/petri/core/v2/provider"
	"github.com/stretchr/testify/assert"
	"testing"
)

var validImageDefinition = provider.ImageDefinition{
	Image: "test",
	GID:   "1000",
	UID:   "1000",
}

func TestImageDefinitionValidation(t *testing.T) {
	tcs := []struct {
		name       string
		def        provider.ImageDefinition
		expectPass bool
	}{
		{
			name: "valid",
			def: provider.ImageDefinition{
				Image: "test",
				GID:   "1000",
				UID:   "1000",
			},
			expectPass: true,
		},
		{
			name: "empty image",
			def: provider.ImageDefinition{
				Image: "",
				GID:   "1000",
				UID:   "1000",
			},
		},
		{
			name: "empty uid",
			def: provider.ImageDefinition{
				Image: "test",
				GID:   "1000",
				UID:   "",
			},
			expectPass: false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.def.ValidateBasic()
			if tc.expectPass {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
			}
		})
	}
}

func TestVolumeDefinitionValidation(t *testing.T) {
	tcs := []struct {
		name       string
		def        provider.VolumeDefinition
		expectPass bool
	}{
		{
			name: "valid",
			def: provider.VolumeDefinition{
				MountPath: "/tmp",
				Name:      "test",
				Size:      "100",
			},
			expectPass: true,
		},
		{
			name: "empty mountpath",
			def: provider.VolumeDefinition{
				MountPath: "",
				Name:      "test",
				Size:      "100",
			},
			expectPass: false,
		},
		{
			name: "empty name",
			def: provider.VolumeDefinition{
				MountPath: "/tmp",
				Name:      "",
				Size:      "100",
			},
			expectPass: false,
		},
		{
			name: "empty size",
			def: provider.VolumeDefinition{
				MountPath: "/tmp",
				Name:      "test",
				Size:      "",
			},
			expectPass: false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.def.ValidateBasic()
			if tc.expectPass {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
			}
		})
	}
}

func TestTaskDefinitionValidation(t *testing.T) {
	tcs := []struct {
		name       string
		def        provider.TaskDefinition
		expectPass bool
	}{
		{
			name: "valid",
			def: provider.TaskDefinition{
				Name:          "test",
				ContainerName: "test",
				Image:         validImageDefinition,
			},
			expectPass: true,
		},
		{
			name: "no name",
			def: provider.TaskDefinition{
				ContainerName: "test",
				Image:         validImageDefinition,
			},
			expectPass: false,
		},
		{
			name: "no container name",
			def: provider.TaskDefinition{
				Name:  "test",
				Image: validImageDefinition,
			},
			expectPass: false,
		},
		{
			name: "no image",
			def: provider.TaskDefinition{
				Name:          "test",
				ContainerName: "test",
			},
			expectPass: false,
		},
		{
			name: "invalid image",
			def: provider.TaskDefinition{
				Name:          "test",
				ContainerName: "test",
				Image: provider.ImageDefinition{
					Image: "",
				},
			},
			expectPass: false,
		},
		{
			name: "invalid port",
			def: provider.TaskDefinition{
				Name:          "test",
				ContainerName: "test",
				Image:         validImageDefinition,
				Ports:         []string{"", "100000"},
			},
			expectPass: false,
		},
		{
			name: "invalid sidecar",
			def: provider.TaskDefinition{
				Name:          "test",
				ContainerName: "test",
				Image:         validImageDefinition,
				Sidecars: []provider.TaskDefinition{
					{
						Name: "test",
					},
				},
			},
			expectPass: false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.def.ValidateBasic()
			if tc.expectPass {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
			}
		})
	}
}
