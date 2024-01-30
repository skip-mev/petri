package provider

type VolumeDefinition struct {
	Name      string
	MountPath string
	Size      string
}

type TaskDefinition struct {
	Name          string // Name is used when generating volumes, etc. - additional resources for the container
	ContainerName string // ContainerName is used for the actual container / job name
	Image         ImageDefinition
	Ports         []string
	Environment   map[string]string
	DataDir       string
	Entrypoint    []string
	Command       []string
	Args          []string
	Sidecars      []TaskDefinition

	ProviderSpecificConfig interface{}
}

type ImageDefinition struct {
	Image string
	UID   string
	GID   string
}
