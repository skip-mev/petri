package digitalocean

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// DockerClient is an interface that abstracts Docker functionality
type DockerClient interface {
	Ping(ctx context.Context) (types.Ping, error)
	ImageInspectWithRaw(ctx context.Context, image string) (types.ImageInspect, []byte, error)
	ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error)
	ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerExecCreate(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error)
	ContainerExecAttach(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error)
	ContainerExecInspect(ctx context.Context, execID string) (container.ExecInspect, error)
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	Close() error
}

type defaultDockerClient struct {
	client *dockerclient.Client
}

func NewDockerClient(host string) (DockerClient, error) {
	client, err := dockerclient.NewClientWithOpts(dockerclient.WithHost(host))
	if err != nil {
		return nil, err
	}
	return &defaultDockerClient{client: client}, nil
}

func (d *defaultDockerClient) Ping(ctx context.Context) (types.Ping, error) {
	return d.client.Ping(ctx)
}

func (d *defaultDockerClient) ImageInspectWithRaw(ctx context.Context, image string) (types.ImageInspect, []byte, error) {
	return d.client.ImageInspectWithRaw(ctx, image)
}

func (d *defaultDockerClient) ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
	return d.client.ImagePull(ctx, ref, options)
}

func (d *defaultDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error) {
	return d.client.ContainerCreate(ctx, config, hostConfig, networkingConfig, platform, containerName)
}

func (d *defaultDockerClient) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	return d.client.ContainerList(ctx, options)
}

func (d *defaultDockerClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	return d.client.ContainerStart(ctx, containerID, options)
}

func (d *defaultDockerClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	return d.client.ContainerStop(ctx, containerID, options)
}

func (d *defaultDockerClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	return d.client.ContainerInspect(ctx, containerID)
}

func (d *defaultDockerClient) ContainerExecCreate(ctx context.Context, container string, options container.ExecOptions) (types.IDResponse, error) {
	return d.client.ContainerExecCreate(ctx, container, options)
}

func (d *defaultDockerClient) ContainerExecAttach(ctx context.Context, execID string, options container.ExecStartOptions) (types.HijackedResponse, error) {
	return d.client.ContainerExecAttach(ctx, execID, options)
}

func (d *defaultDockerClient) ContainerExecInspect(ctx context.Context, execID string) (container.ExecInspect, error) {
	return d.client.ContainerExecInspect(ctx, execID)
}

func (d *defaultDockerClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	return d.client.ContainerRemove(ctx, containerID, options)
}

func (d *defaultDockerClient) Close() error {
	return d.client.Close()
}
