package provider

type VolumeDefinition struct {
	Name      string
	MountPath string
	Size      string
}

type TaskDefinition struct {
	Name        string
	Image       ImageDefinition
	Ports       []string
	Environment map[string]string
	DataDir     string
	Command     []string
	Args        []string
	Sidecars    []TaskDefinition
}

type ImageDefinition struct {
	Image string
	UID   string
	GID   string
}
