package provider

// VolumeDefinition defines the configuration for a volume. Some providers might not support creating a volume,
// but this detail is abstracted away when CreateTask is used
type VolumeDefinition struct {
	Name      string
	MountPath string
	Size      string
}

// TaskDefinition defines the configuration for a task
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

// ImageDefinition defines the details of a specific Docker image
type ImageDefinition struct {
	Image string
	UID   string
	GID   string
}
