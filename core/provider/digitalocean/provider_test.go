package digitalocean

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/skip-mev/petri/core/v2/provider"
	"github.com/skip-mev/petri/core/v2/provider/digitalocean/mocks"
	"github.com/skip-mev/petri/core/v2/util"
)

func setupTestProvider(t *testing.T, ctx context.Context) (*Provider, *mocks.DoClient, *mocks.DockerClient) {
	logger := zap.NewExample()
	mockDO := mocks.NewDoClient(t)
	mockDocker := mocks.NewDockerClient(t)

	mockDocker.On("Ping", ctx).Return(types.Ping{}, nil)
	mockDocker.On("ImageInspectWithRaw", ctx, "ubuntu:latest").Return(types.ImageInspect{}, []byte{}, fmt.Errorf("image not found"))
	mockDocker.On("ImagePull", ctx, "ubuntu:latest", image.PullOptions{}).Return(io.NopCloser(strings.NewReader("")), nil)
	mockDocker.On("ContainerCreate", ctx, &container.Config{
		Image:      "ubuntu:latest",
		Entrypoint: []string{"/bin/bash"},
		Cmd:        []string{"-c", "echo hello"},
		Env:        []string{"TEST=value"},
		Hostname:   "test-task",
		Labels: map[string]string{
			providerLabelName: "test-provider",
		},
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: "/docker_volumes",
				Target: "/data",
			},
		},
		NetworkMode: container.NetworkMode("host"),
	}, (*network.NetworkingConfig)(nil), (*specs.Platform)(nil), "test-container").Return(container.CreateResponse{ID: "test-container"}, nil)

	mockDO.On("CreateTag", ctx, mock.Anything).Return(&godo.Tag{Name: "test-tag"}, nil)
	mockDO.On("CreateFirewall", ctx, mock.Anything).Return(&godo.Firewall{ID: "test-firewall"}, nil)
	mockDO.On("GetKeyByFingerprint", ctx, mock.AnythingOfType("string")).Return(nil, nil)
	mockDO.On("CreateKey", ctx, mock.Anything).Return(&godo.Key{}, nil)

	mockDockerClients := map[string]DockerClient{
		"10.0.0.1": mockDocker,
	}

	p, err := NewProviderWithClient(ctx, logger, "test-provider", mockDO, mockDockerClients, []string{}, nil)
	require.NoError(t, err)

	droplet := &godo.Droplet{
		ID: 123,
		Networks: &godo.Networks{
			V4: []godo.NetworkV4{
				{
					IPAddress: "10.0.0.1",
					Type:      "public",
				},
			},
		},
		Status: "active",
	}

	var callCount int
	mockDO.On("CreateDroplet", ctx, mock.Anything).Return(droplet, nil)
	mockDO.On("GetDroplet", ctx, droplet.ID).Return(func(ctx context.Context, id int) *godo.Droplet {
		if callCount == 0 {
			callCount++
			return &godo.Droplet{
				ID: id,
				Networks: &godo.Networks{
					V4: []godo.NetworkV4{
						{
							IPAddress: "10.0.0.1",
							Type:      "public",
						},
					},
				},
				Status: "new",
			}
		}
		return droplet
	}, func(ctx context.Context, id int) error {
		return nil
	}).Maybe()

	mockDO.On("DeleteDropletByID", ctx, droplet.ID).Return(nil).Maybe()

	return p, mockDO, mockDocker
}

func TestCreateTask_ValidTask(t *testing.T) {
	ctx := context.Background()
	p, _, _ := setupTestProvider(t, ctx)

	taskDef := provider.TaskDefinition{
		Name:          "test-task",
		Image:         provider.ImageDefinition{Image: "ubuntu:latest", UID: "1000", GID: "1000"},
		Entrypoint:    []string{"/bin/bash"},
		Command:       []string{"-c", "echo hello"},
		Environment:   map[string]string{"TEST": "value"},
		DataDir:       "/data",
		ContainerName: "test-container",
		ProviderSpecificConfig: DigitalOceanTaskConfig{
			"size":     "s-1vcpu-1gb",
			"region":   "nyc1",
			"image_id": "123456",
		},
	}

	task, err := p.CreateTask(ctx, taskDef)
	assert.NoError(t, err)
	assert.Equal(t, task.GetDefinition(), taskDef)
	assert.NotNil(t, task)
}

func setupValidationTestProvider(t *testing.T, ctx context.Context) *Provider {
	logger := zap.NewExample()
	mockDO := mocks.NewDoClient(t)
	mockDocker := mocks.NewDockerClient(t)

	mockDO.On("CreateTag", ctx, mock.Anything).Return(&godo.Tag{Name: "test-tag"}, nil)
	mockDO.On("CreateFirewall", ctx, mock.Anything).Return(&godo.Firewall{ID: "test-firewall"}, nil)
	mockDO.On("GetKeyByFingerprint", ctx, mock.AnythingOfType("string")).Return(nil, nil)
	mockDO.On("CreateKey", ctx, mock.Anything).Return(&godo.Key{}, nil)

	mockDockerClients := map[string]DockerClient{
		"10.0.0.1": mockDocker,
	}

	p, err := NewProviderWithClient(ctx, logger, "test-provider", mockDO, mockDockerClients, []string{}, nil)
	require.NoError(t, err)

	return p
}

func TestCreateTask_MissingProviderConfig(t *testing.T) {
	ctx := context.Background()
	p := setupValidationTestProvider(t, ctx)

	taskDef := provider.TaskDefinition{
		Name:                   "test-task",
		Image:                  provider.ImageDefinition{Image: "ubuntu:latest", UID: "1000", GID: "1000"},
		ProviderSpecificConfig: nil,
	}

	task, err := p.CreateTask(ctx, taskDef)
	assert.Error(t, err)
	assert.Nil(t, task)
}

func TestCreateTask_MissingRegion(t *testing.T) {
	ctx := context.Background()
	p := setupValidationTestProvider(t, ctx)

	taskDef := provider.TaskDefinition{
		Name:  "test-task",
		Image: provider.ImageDefinition{Image: "ubuntu:latest", UID: "1000", GID: "1000"},
		ProviderSpecificConfig: DigitalOceanTaskConfig{
			"size":     "s-1vcpu-1gb",
			"image_id": "123456",
		},
	}

	task, err := p.CreateTask(ctx, taskDef)
	assert.Error(t, err)
	assert.Nil(t, task)
}

func TestSerializeAndRestoreTask(t *testing.T) {
	ctx := context.Background()
	p, mockDO, mockDocker := setupTestProvider(t, ctx)

	taskDef := provider.TaskDefinition{
		Name:          "test-task",
		Image:         provider.ImageDefinition{Image: "ubuntu:latest", UID: "1000", GID: "1000"},
		Entrypoint:    []string{"/bin/bash"},
		Command:       []string{"-c", "echo hello"},
		Environment:   map[string]string{"TEST": "value"},
		DataDir:       "/data",
		ContainerName: "test-container",
		ProviderSpecificConfig: DigitalOceanTaskConfig{
			"size":     "s-1vcpu-1gb",
			"region":   "nyc1",
			"image_id": "123456",
		},
	}

	task, err := p.CreateTask(ctx, taskDef)
	require.NoError(t, err)

	taskData, err := p.SerializeTask(ctx, task)
	assert.NoError(t, err)
	assert.NotNil(t, taskData)

	mockDO.On("GetDroplet", ctx, 123).Return(&godo.Droplet{
		ID:     123,
		Name:   "test-droplet",
		Status: "active",
		Networks: &godo.Networks{
			V4: []godo.NetworkV4{
				{
					IPAddress: "10.0.0.1",
					Type:      "public",
				},
			},
		},
	}, nil)

	deserializedTask, err := p.DeserializeTask(ctx, taskData)
	assert.NoError(t, err)
	assert.NotNil(t, deserializedTask)

	t1 := task.(*Task)
	t2 := deserializedTask.(*Task)
	t1State := t1.GetState()
	t2State := t2.GetState()

	if configMap, ok := t2State.Definition.ProviderSpecificConfig.(map[string]interface{}); ok {
		doConfig := make(DigitalOceanTaskConfig)
		for k, v := range configMap {
			doConfig[k] = v.(string)
		}
		t2State.Definition.ProviderSpecificConfig = doConfig
	}

	assert.Equal(t, t1State, t2State)
	assert.NotNil(t, t2.logger)
	assert.NotNil(t, t2State.SSHKeyPair)
	assert.NotNil(t, t2.doClient)
	assert.NotNil(t, t2.dockerClient)
	assert.NotNil(t, t2.provider)

	mockDO.AssertExpectations(t)
	mockDocker.AssertExpectations(t)
}

func TestConcurrentTaskCreationAndCleanup(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	logger, _ := zap.NewDevelopment()
	mockDockerClients := make(map[string]DockerClient)
	mockDO := mocks.NewDoClient(t)

	for i := 0; i < 10; i++ {
		ip := fmt.Sprintf("10.0.0.%d", i+1)
		mockDocker := mocks.NewDockerClient(t)
		mockDockerClients[ip] = mockDocker

		mockDocker.On("Ping", ctx).Return(types.Ping{}, nil).Once()
		mockDocker.On("ImageInspectWithRaw", ctx, "nginx:latest").Return(types.ImageInspect{}, []byte{}, fmt.Errorf("image not found")).Once()
		mockDocker.On("ImagePull", ctx, "nginx:latest", image.PullOptions{}).Return(io.NopCloser(strings.NewReader("")), nil).Once()
		mockDocker.On("ContainerCreate", ctx, mock.MatchedBy(func(config *container.Config) bool {
			return config.Image == "nginx:latest"
		}), mock.Anything, (*network.NetworkingConfig)(nil), (*specs.Platform)(nil), mock.AnythingOfType("string")).Return(container.CreateResponse{ID: fmt.Sprintf("container-%d", i)}, nil).Once()
		mockDocker.On("ContainerStart", ctx, fmt.Sprintf("container-%d", i), container.StartOptions{}).Return(nil).Once()
		mockDocker.On("ContainerList", ctx, container.ListOptions{
			Limit: 1,
		}).Return([]types.Container{
			{
				ID:    fmt.Sprintf("container-%d", i),
				State: "running",
			},
		}, nil).Times(3)
		mockDocker.On("ContainerInspect", ctx, fmt.Sprintf("container-%d", i)).Return(types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				State: &types.ContainerState{
					Status: "running",
				},
			},
		}, nil).Maybe()
		mockDocker.On("Close").Return(nil).Once()
	}

	mockDO.On("CreateTag", ctx, mock.Anything).Return(&godo.Tag{Name: "test-tag"}, nil)

	mockDO.On("CreateFirewall", ctx, mock.Anything).Return(&godo.Firewall{ID: "test-firewall"}, nil)

	mockDO.On("GetKeyByFingerprint", ctx, mock.AnythingOfType("string")).
		Return(nil, nil)
	mockDO.On("CreateKey", ctx, mock.Anything).Return(&godo.Key{}, nil)

	p, err := NewProviderWithClient(ctx, logger, "test-provider", mockDO, mockDockerClients, []string{}, nil)
	require.NoError(t, err)

	numTasks := 10
	var wg sync.WaitGroup
	errors := make(chan error, numTasks)
	tasks := make(chan *Task, numTasks)
	taskMutex := sync.Mutex{}
	dropletIDs := make(map[string]bool)
	ipAddresses := make(map[string]bool)

	for i := 0; i < numTasks; i++ {
		dropletID := 1000 + i
		droplet := &godo.Droplet{
			ID:     dropletID,
			Status: "active",
			Networks: &godo.Networks{
				V4: []godo.NetworkV4{
					{
						Type:      "public",
						IPAddress: fmt.Sprintf("10.0.0.%d", i+1),
					},
				},
			},
		}

		mockDO.On("CreateDroplet", ctx, mock.Anything).Return(droplet, nil).Once()
		// we cant predict how many times GetDroplet will be called exactly as the provider polls waiting for its creation
		mockDO.On("GetDroplet", ctx, dropletID).Return(droplet, nil).Maybe()
		mockDO.On("DeleteDropletByID", ctx, dropletID).Return(nil).Once()
	}

	mockDO.On("DeleteDropletByTag", ctx, mock.AnythingOfType("string")).Return(nil).Once()
	mockDO.On("DeleteFirewall", ctx, mock.AnythingOfType("string")).Return(nil).Once()
	mockDO.On("DeleteKeyByFingerprint", ctx, mock.AnythingOfType("string")).Return(nil).Once()
	mockDO.On("DeleteTag", ctx, mock.AnythingOfType("string")).Return(nil).Once()

	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			task, err := p.CreateTask(ctx, provider.TaskDefinition{
				Name:          fmt.Sprintf("test-task-%d", index),
				ContainerName: fmt.Sprintf("test-container-%d", index),
				Image: provider.ImageDefinition{
					Image: "nginx:latest",
					UID:   "1000",
					GID:   "1000",
				},
				Ports: []string{"80"},
				ProviderSpecificConfig: DigitalOceanTaskConfig{
					"size":     "s-1vcpu-1gb",
					"region":   "nyc1",
					"image_id": "123456789",
				},
			})

			if err != nil {
				errors <- fmt.Errorf("task creation error: %v", err)
				return
			}

			if err := task.Start(ctx); err != nil {
				errors <- fmt.Errorf("task start error: %v", err)
				return
			}

			taskMutex.Lock()
			doTask := task.(*Task)
			state := doTask.GetState()

			if dropletIDs[state.ID] {
				errors <- fmt.Errorf("duplicate droplet ID found: %s", state.ID)
			}
			dropletIDs[state.ID] = true

			ip, err := task.GetIP(ctx)
			if err == nil {
				if ipAddresses[ip] {
					errors <- fmt.Errorf("duplicate IP address found: %s", ip)
				}
				ipAddresses[ip] = true
			}

			tasks <- doTask
			taskMutex.Unlock()
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		require.NoError(t, err)
	}

	require.Equal(t, numTasks, len(p.GetState().TaskStates), "Provider state should contain all tasks")

	var tasksToCleanup []*Task
	close(tasks)
	for task := range tasks {
		status, err := task.GetStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, provider.TASK_RUNNING, status, "All tasks should be in running state")

		state := task.GetState()
		require.NotEmpty(t, state.ID, "Task should have a droplet ID")
		require.NotEmpty(t, state.Name, "Task should have a name")
		require.NotEmpty(t, state.Definition.ContainerName, "Task should have a container name")
		tasksToCleanup = append(tasksToCleanup, task)
	}

	var cleanupWg sync.WaitGroup
	cleanupErrors := make(chan error, numTasks)

	for _, task := range tasksToCleanup {
		cleanupWg.Add(1)
		go func(t *Task) {
			defer cleanupWg.Done()
			if err := t.Destroy(ctx); err != nil {
				cleanupErrors <- fmt.Errorf("cleanup error: %v", err)
				return
			}
			if err != nil {
				cleanupErrors <- fmt.Errorf("task state cleanup error: %v", err)
			}
		}(task)
	}

	cleanupWg.Wait()
	close(cleanupErrors)

	for err := range cleanupErrors {
		require.NoError(t, err)
	}

	err = util.WaitForCondition(ctx, 30*time.Second, 100*time.Millisecond, func() (bool, error) {
		return len(p.GetState().TaskStates) == 0, nil
	})
	require.NoError(t, err, "Provider state should be empty after cleanup")

	err = p.Teardown(ctx)
	require.NoError(t, err)

	mockDO.AssertExpectations(t)
	for _, client := range mockDockerClients {
		client.(*mocks.DockerClient).AssertExpectations(t)
	}
}

func TestProviderSerialization(t *testing.T) {
	ctx := context.Background()
	mockDO := mocks.NewDoClient(t)
	mockDocker := mocks.NewDockerClient(t)

	mockDO.On("CreateTag", ctx, mock.Anything).Return(&godo.Tag{Name: "petri-droplet-test"}, nil)
	mockDO.On("CreateFirewall", ctx, mock.Anything).Return(&godo.Firewall{ID: "test-firewall"}, nil)
	mockDO.On("GetKeyByFingerprint", ctx, mock.AnythingOfType("string")).Return(nil, nil)
	mockDO.On("CreateKey", ctx, mock.Anything).Return(&godo.Key{}, nil)

	mockDockerClients := map[string]DockerClient{
		"10.0.0.1": mockDocker,
	}

	p1, err := NewProviderWithClient(ctx, zap.NewExample(), "test-provider", mockDO, mockDockerClients, []string{}, nil)
	require.NoError(t, err)

	droplet := &godo.Droplet{
		ID: 123,
		Networks: &godo.Networks{
			V4: []godo.NetworkV4{
				{
					IPAddress: "10.0.0.1",
					Type:      "public",
				},
			},
		},
		Status: "active",
	}

	mockDO.On("CreateDroplet", ctx, mock.Anything).Return(droplet, nil)
	mockDO.On("GetDroplet", ctx, droplet.ID).Return(droplet, nil).Maybe()

	mockDocker.On("Ping", ctx).Return(types.Ping{}, nil).Maybe()
	mockDocker.On("ImageInspectWithRaw", ctx, "ubuntu:latest").Return(types.ImageInspect{}, []byte{}, fmt.Errorf("image not found"))
	mockDocker.On("ImagePull", ctx, "ubuntu:latest", image.PullOptions{}).Return(io.NopCloser(strings.NewReader("")), nil)
	mockDocker.On("ContainerCreate", ctx, mock.MatchedBy(func(config *container.Config) bool {
		return config.Image == "ubuntu:latest" &&
			config.Hostname == "test-task" &&
			len(config.Labels) > 0 &&
			config.Labels[providerLabelName] == "test-provider"
	}), mock.MatchedBy(func(hostConfig *container.HostConfig) bool {
		return len(hostConfig.Mounts) == 1 &&
			hostConfig.Mounts[0].Target == "/data" &&
			hostConfig.NetworkMode == "host"
	}), mock.Anything, mock.Anything, mock.Anything).Return(container.CreateResponse{ID: "test-container"}, nil)

	_, err = p1.CreateTask(ctx, provider.TaskDefinition{
		Name:          "test-task",
		ContainerName: "test-container",
		Image: provider.ImageDefinition{
			Image: "ubuntu:latest",
			UID:   "1000",
			GID:   "1000",
		},
		DataDir: "/data",
		ProviderSpecificConfig: DigitalOceanTaskConfig{
			"size":     "s-1vcpu-1gb",
			"region":   "nyc1",
			"image_id": "123456",
		},
	})
	require.NoError(t, err)

	state1 := p1.GetState()
	serialized, err := p1.SerializeProvider(ctx)
	require.NoError(t, err)

	mockDO2 := mocks.NewDoClient(t)
	mockDO2.On("GetDroplet", ctx, droplet.ID).Return(droplet, nil).Maybe()

	mockDocker2 := mocks.NewDockerClient(t)
	mockDocker2.On("Ping", ctx).Return(types.Ping{}, nil).Maybe()

	mockDockerClients2 := map[string]DockerClient{
		"10.0.0.1": mockDocker2,
	}

	p2, err := RestoreProvider(ctx, "test-token", serialized, mockDO2, mockDockerClients2)
	require.NoError(t, err)

	state2 := p2.GetState()
	assert.Equal(t, state1.Name, state2.Name)
	assert.Equal(t, state1.PetriTag, state2.PetriTag)
	assert.Equal(t, state1.FirewallID, state2.FirewallID)
	assert.Equal(t, len(state1.TaskStates), len(state2.TaskStates))

	for id, task1 := range state1.TaskStates {
		task2, exists := state2.TaskStates[id]
		assert.True(t, exists)
		assert.Equal(t, task1.Name, task2.Name)
		assert.Equal(t, task1.Status, task2.Status)

		if configMap, ok := task2.Definition.ProviderSpecificConfig.(map[string]interface{}); ok {
			doConfig := make(DigitalOceanTaskConfig)
			for k, v := range configMap {
				doConfig[k] = v.(string)
			}
			task2.Definition.ProviderSpecificConfig = doConfig
		}
		assert.Equal(t, task1.Definition, task2.Definition)
	}
}
