package digitalocean

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/skip-mev/petri/core/v2/provider"
	"github.com/skip-mev/petri/core/v2/provider/digitalocean/mocks"
	"github.com/skip-mev/petri/core/v2/util"
)

func setupMockClient(t *testing.T) *mocks.DoClient {
	return mocks.NewDoClient(t)
}

func setupMockDockerClient(t *testing.T) *mocks.DockerClient {
	return mocks.NewDockerClient(t)
}

func TestNewProvider(t *testing.T) {
	logger := zap.NewExample()
	ctx := context.Background()

	tests := []struct {
		name          string
		token         string
		additionalIPs []string
		sshKeyPair    *SSHKeyPair
		expectedError bool
		mockSetup     func(*mocks.DoClient)
	}{
		{
			name:          "valid provider creation",
			token:         "test-token",
			additionalIPs: []string{"1.2.3.4"},
			sshKeyPair:    nil,
			expectedError: false,
			mockSetup: func(m *mocks.DoClient) {
				m.On("CreateTag", mock.Anything, mock.AnythingOfType("*godo.TagCreateRequest")).
					Return(&godo.Tag{Name: "test-tag"}, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

				m.On("CreateFirewall", mock.Anything, mock.AnythingOfType("*godo.FirewallRequest")).
					Return(&godo.Firewall{ID: "test-firewall"}, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

				m.On("GetKeyByFingerprint", mock.Anything, mock.AnythingOfType("string")).
					Return(nil, &godo.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}, nil)
				m.On("CreateKey", mock.Anything, mock.AnythingOfType("*godo.KeyCreateRequest")).
					Return(&godo.Key{}, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)
			},
		},
		{
			name:          "bad token",
			token:         "foobar",
			additionalIPs: []string{},
			sshKeyPair:    nil,
			expectedError: true,
			mockSetup: func(m *mocks.DoClient) {
				m.On("CreateTag", mock.Anything, mock.AnythingOfType("*godo.TagCreateRequest")).
					Return(nil, &godo.Response{Response: &http.Response{StatusCode: http.StatusUnauthorized}}, fmt.Errorf("unauthorized"))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := setupMockClient(t)
			mockDocker := setupMockDockerClient(t)
			tc.mockSetup(mockClient)
			mockDockerClients := map[string]DockerClient{
				"test-ip": mockDocker,
			}

			provider, err := NewProviderWithClient(ctx, logger, "test-provider", mockClient, mockDockerClients, tc.additionalIPs, tc.sshKeyPair)
			if tc.expectedError {
				assert.Error(t, err)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
				assert.NotEmpty(t, provider.petriTag)
				assert.NotEmpty(t, provider.firewallID)
			}
		})
	}
}

func TestCreateTask(t *testing.T) {
	logger := zap.NewExample()
	ctx := context.Background()
	mockClient := setupMockClient(t)
	mockDocker := setupMockDockerClient(t)

	mockDocker.On("Ping", mock.Anything).Return(types.Ping{}, nil)
	mockDocker.On("ImageInspectWithRaw", mock.Anything, mock.AnythingOfType("string")).Return(types.ImageInspect{}, []byte{}, fmt.Errorf("image not found"))
	mockDocker.On("ImagePull", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("image.PullOptions")).Return(io.NopCloser(strings.NewReader("")), nil)
	mockDocker.On("ContainerCreate", mock.Anything, mock.AnythingOfType("*container.Config"), mock.AnythingOfType("*container.HostConfig"), mock.AnythingOfType("*network.NetworkingConfig"), mock.AnythingOfType("*v1.Platform"), mock.AnythingOfType("string")).Return(container.CreateResponse{ID: "test-container"}, nil)

	mockClient.On("CreateTag", mock.Anything, mock.AnythingOfType("*godo.TagCreateRequest")).
		Return(&godo.Tag{Name: "test-tag"}, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)
	mockClient.On("CreateFirewall", mock.Anything, mock.AnythingOfType("*godo.FirewallRequest")).
		Return(&godo.Firewall{ID: "test-firewall"}, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)
	mockClient.On("GetKeyByFingerprint", mock.Anything, mock.AnythingOfType("string")).
		Return(nil, &godo.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}, nil)
	mockClient.On("CreateKey", mock.Anything, mock.AnythingOfType("*godo.KeyCreateRequest")).
		Return(&godo.Key{}, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)
	mockDockerClients := map[string]DockerClient{
		"10.0.0.1": mockDocker,
	}

	p, err := NewProviderWithClient(ctx, logger, "test-provider", mockClient, mockDockerClients, []string{}, nil)
	require.NoError(t, err)

	mockClient.On("CreateDroplet", mock.Anything, mock.AnythingOfType("*godo.DropletCreateRequest")).
		Return(&godo.Droplet{
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
		}, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

	mockClient.On("GetDroplet", mock.Anything, mock.AnythingOfType("int")).
		Return(&godo.Droplet{
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
		}, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

	tests := []struct {
		name          string
		taskDef       provider.TaskDefinition
		expectedError bool
	}{
		{
			name: "valid task creation",
			taskDef: provider.TaskDefinition{
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
			},
			expectedError: false,
		},
		{
			name: "missing provider specific config",
			taskDef: provider.TaskDefinition{
				Name:                   "test-task",
				Image:                  provider.ImageDefinition{Image: "ubuntu:latest", UID: "1000", GID: "1000"},
				ProviderSpecificConfig: nil,
			},
			expectedError: true,
		},
		{
			name: "invalid provider specific config - missing region",
			taskDef: provider.TaskDefinition{
				Name:  "test-task",
				Image: provider.ImageDefinition{Image: "ubuntu:latest", UID: "1000", GID: "1000"},
				ProviderSpecificConfig: DigitalOceanTaskConfig{
					"size":     "s-1vcpu-1gb",
					"image_id": "123456",
				},
			},
			expectedError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task, err := p.CreateTask(ctx, tc.taskDef)
			if tc.expectedError {
				assert.Error(t, err)
				assert.Nil(t, task)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, task)
			}
		})
	}
}

func TestSerializeAndRestore(t *testing.T) {
	logger := zap.NewExample()
	ctx := context.Background()
	mockClient := setupMockClient(t)
	mockDocker := setupMockDockerClient(t)
	mockDockerClients := map[string]DockerClient{
		"10.0.0.1": mockDocker,
	}
	mockDocker.On("Ping", mock.Anything).Return(types.Ping{}, nil)
	mockDocker.On("ImageInspectWithRaw", mock.Anything, mock.AnythingOfType("string")).Return(types.ImageInspect{}, []byte{}, fmt.Errorf("image not found"))
	mockDocker.On("ImagePull", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("image.PullOptions")).Return(io.NopCloser(strings.NewReader("")), nil)
	mockDocker.On("ContainerCreate", mock.Anything, mock.AnythingOfType("*container.Config"), mock.AnythingOfType("*container.HostConfig"), mock.AnythingOfType("*network.NetworkingConfig"), mock.AnythingOfType("*v1.Platform"), mock.AnythingOfType("string")).Return(container.CreateResponse{ID: "test-container"}, nil)

	mockClient.On("CreateTag", mock.Anything, mock.AnythingOfType("*godo.TagCreateRequest")).
		Return(&godo.Tag{Name: "test-tag"}, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)
	mockClient.On("CreateFirewall", mock.Anything, mock.AnythingOfType("*godo.FirewallRequest")).
		Return(&godo.Firewall{ID: "test-firewall"}, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)
	mockClient.On("GetKeyByFingerprint", mock.Anything, mock.AnythingOfType("string")).
		Return(nil, &godo.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}, nil)
	mockClient.On("CreateKey", mock.Anything, mock.AnythingOfType("*godo.KeyCreateRequest")).
		Return(&godo.Key{}, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

	mockClient.On("CreateDroplet", mock.Anything, mock.AnythingOfType("*godo.DropletCreateRequest")).
		Return(&godo.Droplet{
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
		}, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

	mockClient.On("GetDroplet", mock.Anything, mock.AnythingOfType("int")).
		Return(&godo.Droplet{
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
		}, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

	p, err := NewProviderWithClient(ctx, logger, "test-provider", mockClient, mockDockerClients, []string{}, nil)
	require.NoError(t, err)

	providerData, err := p.SerializeProvider(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, providerData)

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

	deserializedTask, err := p.DeserializeTask(ctx, taskData)
	assert.NoError(t, err)
	assert.NotNil(t, deserializedTask)
}

func TestTeardown(t *testing.T) {
	logger := zap.NewExample()
	ctx := context.Background()
	mockClient := setupMockClient(t)
	mockDocker := setupMockDockerClient(t)

	mockClient.On("CreateTag", mock.Anything, mock.AnythingOfType("*godo.TagCreateRequest")).
		Return(&godo.Tag{Name: "test-tag"}, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)
	mockClient.On("CreateFirewall", mock.Anything, mock.AnythingOfType("*godo.FirewallRequest")).
		Return(&godo.Firewall{ID: "test-firewall"}, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)
	mockClient.On("GetKeyByFingerprint", mock.Anything, mock.AnythingOfType("string")).
		Return(nil, &godo.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}, nil)
	mockClient.On("CreateKey", mock.Anything, mock.AnythingOfType("*godo.KeyCreateRequest")).
		Return(&godo.Key{}, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

	mockClient.On("DeleteDropletByTag", mock.Anything, mock.AnythingOfType("string")).
		Return(&godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)
	mockClient.On("DeleteFirewall", mock.Anything, mock.AnythingOfType("string")).
		Return(&godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)
	mockClient.On("DeleteKeyByFingerprint", mock.Anything, mock.AnythingOfType("string")).
		Return(&godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)
	mockClient.On("DeleteTag", mock.Anything, mock.AnythingOfType("string")).
		Return(&godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

	p, err := NewProviderWithClient(ctx, logger, "test-provider", mockClient, nil, []string{}, nil)
	require.NoError(t, err)

	err = p.Teardown(ctx)
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
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

		mockDocker.On("Ping", mock.Anything).Return(types.Ping{}, nil).Once()
		mockDocker.On("ImageInspectWithRaw", mock.Anything, mock.AnythingOfType("string")).Return(types.ImageInspect{}, []byte{}, fmt.Errorf("image not found")).Once()
		mockDocker.On("ImagePull", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("image.PullOptions")).Return(io.NopCloser(strings.NewReader("")), nil).Once()
		mockDocker.On("ContainerCreate", mock.Anything, mock.AnythingOfType("*container.Config"), mock.AnythingOfType("*container.HostConfig"),
			mock.AnythingOfType("*network.NetworkingConfig"), mock.AnythingOfType("*v1.Platform"),
			mock.AnythingOfType("string")).Return(container.CreateResponse{ID: fmt.Sprintf("container-%d", i)}, nil).Once()
		mockDocker.On("ContainerStart", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockDocker.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{
			{
				ID:    fmt.Sprintf("container-%d", i),
				State: "running",
			},
		}, nil).Times(3)
		mockDocker.On("ContainerInspect", mock.Anything, mock.Anything).Return(types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				State: &types.ContainerState{
					Status: "running",
				},
			},
		}, nil).Maybe()
		mockDocker.On("Close").Return(nil).Once()
	}

	mockDO.On("CreateTag", mock.Anything, mock.AnythingOfType("*godo.TagCreateRequest")).
		Return(&godo.Tag{Name: "test-tag"}, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)
	mockDO.On("CreateFirewall", mock.Anything, mock.AnythingOfType("*godo.FirewallRequest")).
		Return(&godo.Firewall{ID: "test-firewall"}, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)
	mockDO.On("GetKeyByFingerprint", mock.Anything, mock.AnythingOfType("string")).
		Return(nil, &godo.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}, nil)
	mockDO.On("CreateKey", mock.Anything, mock.AnythingOfType("*godo.KeyCreateRequest")).
		Return(&godo.Key{}, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

	p, err := NewProviderWithClient(ctx, logger, "test-provider", mockDO, mockDockerClients, []string{}, nil)
	require.NoError(t, err)

	numTasks := 10
	var wg sync.WaitGroup
	errors := make(chan error, numTasks)
	tasks := make(chan *Task, numTasks)
	taskMutex := sync.Mutex{}
	dropletIDs := make(map[int]bool)
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

		mockDO.On("CreateDroplet", mock.Anything, mock.AnythingOfType("*godo.DropletCreateRequest")).Return(droplet, &godo.Response{Response: &http.Response{StatusCode: http.StatusCreated}}, nil).Once()
		// we cant predict how many times GetDroplet will be called exactly as the provider polls waiting for its creation
		mockDO.On("GetDroplet", mock.Anything, dropletID).Return(droplet, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil).Maybe()
		mockDO.On("DeleteDropletByID", mock.Anything, dropletID).Return(&godo.Response{Response: &http.Response{StatusCode: http.StatusNoContent}}, nil).Once()
	}

	// these are called once per provider, not per task
	mockDO.On("DeleteDropletByTag", mock.Anything, mock.AnythingOfType("string")).
		Return(&godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil).Once()
	mockDO.On("DeleteFirewall", mock.Anything, mock.AnythingOfType("string")).
		Return(&godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil).Once()
	mockDO.On("DeleteKeyByFingerprint", mock.Anything, mock.AnythingOfType("string")).
		Return(&godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil).Once()
	mockDO.On("DeleteTag", mock.Anything, mock.AnythingOfType("string")).
		Return(&godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil).Once()

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
					"image_id": "123456789", // Using a string for image_id
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

			// thread-safe recording of task details
			taskMutex.Lock()
			doTask := task.(*Task)
			state := doTask.GetState()

			// check for duplicate droplet IDs or IP addresses
			if dropletIDs[state.ID] {
				errors <- fmt.Errorf("duplicate droplet ID found: %d", state.ID)
			}
			dropletIDs[state.ID] = true

			ip, err := task.GetIP(ctx)
			if err == nil {
				if ipAddresses[ip] {
					errors <- fmt.Errorf("duplicate IP address found: %s", ip)
				}
				ipAddresses[ip] = true
			}

			taskMutex.Unlock()

			tasks <- doTask
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any task creation errors before proceeding
	for err := range errors {
		require.NoError(t, err)
	}

	require.Equal(t, numTasks, len(p.state.TaskStates), "Provider state should contain all tasks")

	// Collect all tasks in a slice before closing the channel
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

	// test cleanup
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

			// Wait for the task to be removed from the provider state
			err = util.WaitForCondition(ctx, 30*time.Second, 100*time.Millisecond, func() (bool, error) {
				taskMutex.Lock()
				defer taskMutex.Unlock()
				_, exists := p.state.TaskStates[t.GetState().ID]
				return !exists, nil
			})
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

	// Wait for provider state to be empty
	err = util.WaitForCondition(ctx, 30*time.Second, 100*time.Millisecond, func() (bool, error) {
		return len(p.state.TaskStates) == 0, nil
	})
	require.NoError(t, err, "Provider state should be empty after cleanup")

	// Teardown the provider to clean up resources
	err = p.Teardown(ctx)
	require.NoError(t, err)

	mockDO.AssertExpectations(t)
	for _, client := range mockDockerClients {
		client.(*mocks.DockerClient).AssertExpectations(t)
	}
}
