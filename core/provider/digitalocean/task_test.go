package digitalocean

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/skip-mev/petri/core/v2/provider"
	"github.com/skip-mev/petri/core/v2/provider/digitalocean/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockConn implements net.Conn interface for testing
type mockConn struct {
	*bytes.Buffer
}

// mockNotFoundError implements errdefs.ErrNotFound interface for testing
type mockNotFoundError struct {
	error
}

func (e mockNotFoundError) NotFound() {}

func (m mockConn) Close() error                       { return nil }
func (m mockConn) LocalAddr() net.Addr                { return nil }
func (m mockConn) RemoteAddr() net.Addr               { return nil }
func (m mockConn) SetDeadline(t time.Time) error      { return nil }
func (m mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m mockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestTaskLifecycle(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	mockDocker := mocks.NewDockerClient(t)
	mockDO := mocks.NewDoClient(t)

	droplet := &godo.Droplet{
		ID:     123,
		Status: "active",
		Networks: &godo.Networks{
			V4: []godo.NetworkV4{
				{
					Type:      "public",
					IPAddress: "1.2.3.4",
				},
			},
		},
	}

	mockDO.On("GetDroplet", mock.Anything, droplet.ID).Return(droplet, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

	container := types.Container{
		ID: "test-container-id",
	}
	mockDocker.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{container}, nil)
	mockDocker.On("ContainerInspect", mock.Anything, mock.Anything).Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State: &types.ContainerState{
				Status: "running",
			},
		},
	}, nil)
	mockDocker.On("ContainerStart", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockDocker.On("ContainerStop", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	task := &Task{
		state: &TaskState{
			ID:           droplet.ID,
			Name:         "test-task",
			ProviderName: "test-provider",
			Definition: provider.TaskDefinition{
				Name: "test-task",
				Image: provider.ImageDefinition{
					Image: "nginx:latest",
					UID:   "1000",
					GID:   "1000",
				},
			},
			Status: provider.TASK_STOPPED,
		},
		logger:       logger,
		dockerClient: mockDocker,
		doClient:     mockDO,
	}

	err := task.Start(ctx)
	require.NoError(t, err)
	require.Equal(t, provider.TASK_RUNNING, task.GetState().Status)

	status, err := task.GetStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, provider.TASK_RUNNING, status)

	err = task.Stop(ctx)
	require.NoError(t, err)
	require.Equal(t, provider.TASK_STOPPED, task.GetState().Status)

	mockDocker.AssertExpectations(t)
	mockDO.AssertExpectations(t)
}

func TestTaskRunCommand(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	mockDocker := mocks.NewDockerClient(t)
	mockDO := mocks.NewDoClient(t)

	testContainer := types.Container{
		ID: "test-testContainer-id",
	}
	mockDocker.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{testContainer}, nil)

	execCreateResp := types.IDResponse{ID: "test-exec-id"}
	mockDocker.On("ContainerExecCreate", mock.Anything, mock.Anything, mock.Anything).Return(execCreateResp, nil)

	conn := &mockConn{Buffer: bytes.NewBuffer([]byte{})}
	mockDocker.On("ContainerExecAttach", mock.Anything, mock.Anything, mock.Anything).Return(types.HijackedResponse{
		Conn:   conn,
		Reader: bufio.NewReader(conn),
	}, nil)
	mockDocker.On("ContainerExecInspect", mock.Anything, mock.Anything).Return(container.ExecInspect{
		ExitCode: 0,
		Running:  false,
	}, nil)

	task := &Task{
		state: &TaskState{
			ID:           1,
			Name:         "test-task",
			ProviderName: "test-provider",
			Definition: provider.TaskDefinition{
				Name: "test-task",
				Image: provider.ImageDefinition{
					Image: "nginx:latest",
					UID:   "1000",
					GID:   "1000",
				},
			},
		},
		logger:       logger,
		dockerClient: mockDocker,
		doClient:     mockDO,
	}

	_, stderr, exitCode, err := task.RunCommand(ctx, []string{"echo", "hello"})
	require.NoError(t, err)
	require.Equal(t, 0, exitCode)
	require.Empty(t, stderr)

	mockDocker.AssertExpectations(t)
}

func TestTaskRunCommandWhileStopped(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	mockDocker := mocks.NewDockerClient(t)
	mockDO := mocks.NewDoClient(t)

	createResp := container.CreateResponse{ID: "test-container-id"}
	mockDocker.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(createResp, nil)
	mockDocker.On("ContainerStart", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mockDocker.On("ContainerInspect", mock.Anything, mock.Anything).Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State: &types.ContainerState{
				Running: true,
			},
		},
	}, nil).Once()

	execCreateResp := types.IDResponse{ID: "test-exec-id"}
	mockDocker.On("ContainerExecCreate", mock.Anything, mock.Anything, mock.Anything).Return(execCreateResp, nil)

	conn := &mockConn{Buffer: bytes.NewBuffer([]byte{})}
	mockDocker.On("ContainerExecAttach", mock.Anything, mock.Anything, mock.Anything).Return(types.HijackedResponse{
		Conn:   conn,
		Reader: bufio.NewReader(conn),
	}, nil)
	mockDocker.On("ContainerExecInspect", mock.Anything, mock.Anything).Return(container.ExecInspect{
		ExitCode: 0,
		Running:  false,
	}, nil)

	mockDocker.On("ContainerInspect", mock.Anything, mock.Anything).Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State: &types.ContainerState{
				Running: false,
			},
		},
	}, nil).Once()

	mockDocker.On("ContainerRemove", mock.Anything, mock.Anything, container.RemoveOptions{Force: true}).Return(nil)

	task := &Task{
		state: &TaskState{
			ID:           1,
			Name:         "test-task",
			ProviderName: "test-provider",
			Definition: provider.TaskDefinition{
				Name: "test-task",
				Image: provider.ImageDefinition{
					Image: "nginx:latest",
					UID:   "1000",
					GID:   "1000",
				},
				ContainerName: "test-task-container",
			},
		},
		logger:       logger,
		dockerClient: mockDocker,
		doClient:     mockDO,
	}

	_, stderr, exitCode, err := task.RunCommandWhileStopped(ctx, []string{"echo", "hello"})
	require.NoError(t, err)
	require.Equal(t, 0, exitCode)
	require.Empty(t, stderr)

	mockDocker.AssertExpectations(t)
}

func TestTaskGetIP(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	mockDocker := mocks.NewDockerClient(t)
	mockDO := mocks.NewDoClient(t)

	expectedIP := "1.2.3.4"
	droplet := &godo.Droplet{
		ID:     123,
		Status: "active",
		Networks: &godo.Networks{
			V4: []godo.NetworkV4{
				{
					Type:      "public",
					IPAddress: expectedIP,
				},
			},
		},
	}

	mockDO.On("GetDroplet", ctx, droplet.ID).Return(droplet, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

	task := &Task{
		state: &TaskState{
			ID:           droplet.ID,
			Name:         "test-task",
			ProviderName: "test-provider",
		},
		logger:       logger,
		dockerClient: mockDocker,
		doClient:     mockDO,
	}

	ip, err := task.GetIP(ctx)
	require.NoError(t, err)
	require.Equal(t, expectedIP, ip)

	externalAddr, err := task.GetExternalAddress(ctx, "80")
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("%s:80", expectedIP), externalAddr)

	mockDO.AssertExpectations(t)
}

func TestTaskDestroy(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	mockDocker := mocks.NewDockerClient(t)
	mockDO := mocks.NewDoClient(t)

	droplet := &godo.Droplet{
		ID:     123,
		Status: "active",
	}

	mockDO.On("GetDroplet", mock.Anything, droplet.ID).Return(droplet, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)
	mockDO.On("DeleteDropletByID", mock.Anything, droplet.ID).Return(&godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)
	mockDocker.On("Close").Return(nil)

	provider := &Provider{
		state: &ProviderState{
			TaskStates: make(map[int]*TaskState),
		},
	}

	task := &Task{
		state: &TaskState{
			ID:           droplet.ID,
			Name:         "test-task",
			ProviderName: "test-provider",
		},
		logger:       logger,
		dockerClient: mockDocker,
		doClient:     mockDO,
		provider:     provider,
	}

	provider.state.TaskStates[task.state.ID] = task.state

	err := task.Destroy(ctx)
	require.NoError(t, err)
	require.Empty(t, provider.state.TaskStates)

	mockDocker.AssertExpectations(t)
	mockDO.AssertExpectations(t)
}

func TestRunCommandWhileStoppedContainerCleanup(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	mockDocker := mocks.NewDockerClient(t)
	mockDO := mocks.NewDoClient(t)

	createResp := container.CreateResponse{ID: "test-container-id"}
	mockDocker.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(createResp, nil)
	mockDocker.On("ContainerStart", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// first ContainerInspect for startup check
	mockDocker.On("ContainerInspect", mock.Anything, mock.Anything).Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State: &types.ContainerState{
				Running: true,
			},
		},
	}, nil).Once()

	execCreateResp := types.IDResponse{ID: "test-exec-id"}
	mockDocker.On("ContainerExecCreate", mock.Anything, mock.Anything, mock.Anything).Return(execCreateResp, nil)

	conn := &mockConn{Buffer: bytes.NewBuffer([]byte{})}
	mockDocker.On("ContainerExecAttach", mock.Anything, mock.Anything, mock.Anything).Return(types.HijackedResponse{
		Conn:   conn,
		Reader: bufio.NewReader(conn),
	}, nil)
	mockDocker.On("ContainerExecInspect", mock.Anything, mock.Anything).Return(container.ExecInspect{
		ExitCode: 0,
		Running:  false,
	}, nil)

	// second ContainerInspect for cleanup check - container exists
	mockDocker.On("ContainerInspect", mock.Anything, mock.Anything).Return(types.ContainerJSON{}, nil).Once()

	// container should be removed since it exists
	mockDocker.On("ContainerRemove", mock.Anything, mock.Anything, container.RemoveOptions{Force: true}).Return(nil)

	task := &Task{
		state: &TaskState{
			ID:           1,
			Name:         "test-task",
			ProviderName: "test-provider",
			Definition: provider.TaskDefinition{
				Name: "test-task",
				Image: provider.ImageDefinition{
					Image: "nginx:latest",
					UID:   "1000",
					GID:   "1000",
				},
				ContainerName: "test-task-container",
			},
		},
		logger:       logger,
		dockerClient: mockDocker,
		doClient:     mockDO,
	}

	_, stderr, exitCode, err := task.RunCommandWhileStopped(ctx, []string{"echo", "hello"})
	require.NoError(t, err)
	require.Equal(t, 0, exitCode)
	require.Empty(t, stderr)

	mockDocker.AssertExpectations(t)
}

// this tests the case where the docker container is auto removed before cleanup doesnt return an error
func TestRunCommandWhileStoppedContainerAutoRemoved(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	mockDocker := mocks.NewDockerClient(t)
	mockDO := mocks.NewDoClient(t)

	createResp := container.CreateResponse{ID: "test-container-id"}
	mockDocker.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(createResp, nil)
	mockDocker.On("ContainerStart", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// first ContainerInspect for startup check
	mockDocker.On("ContainerInspect", mock.Anything, mock.Anything).Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State: &types.ContainerState{
				Running: true,
			},
		},
	}, nil).Once()

	execCreateResp := types.IDResponse{ID: "test-exec-id"}
	mockDocker.On("ContainerExecCreate", mock.Anything, mock.Anything, mock.Anything).Return(execCreateResp, nil)

	conn := &mockConn{Buffer: bytes.NewBuffer([]byte{})}
	mockDocker.On("ContainerExecAttach", mock.Anything, mock.Anything, mock.Anything).Return(types.HijackedResponse{
		Conn:   conn,
		Reader: bufio.NewReader(conn),
	}, nil)
	mockDocker.On("ContainerExecInspect", mock.Anything, mock.Anything).Return(container.ExecInspect{
		ExitCode: 0,
		Running:  false,
	}, nil)

	// second ContainerInspect for cleanup check - container not found, so ContainerRemove should not be called
	mockDocker.On("ContainerInspect", mock.Anything, mock.Anything).Return(types.ContainerJSON{}, mockNotFoundError{fmt.Errorf("Error: No such container: test-container-id")}).Once()
	mockDocker.AssertNotCalled(t, "ContainerRemove", mock.Anything, mock.Anything, mock.Anything)

	task := &Task{
		state: &TaskState{
			ID:           1,
			Name:         "test-task",
			ProviderName: "test-provider",
			Definition: provider.TaskDefinition{
				Name: "test-task",
				Image: provider.ImageDefinition{
					Image: "nginx:latest",
					UID:   "1000",
					GID:   "1000",
				},
				ContainerName: "test-task-container",
			},
		},
		logger:       logger,
		dockerClient: mockDocker,
		doClient:     mockDO,
	}

	_, stderr, exitCode, err := task.RunCommandWhileStopped(ctx, []string{"echo", "hello"})
	require.NoError(t, err)
	require.Equal(t, 0, exitCode)
	require.Empty(t, stderr)

	mockDocker.AssertExpectations(t)
}

func TestTaskExposingPort(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	mockDocker := mocks.NewDockerClient(t)
	mockDO := mocks.NewDoClient(t)

	droplet := &godo.Droplet{
		ID:     123,
		Status: "active",
		Networks: &godo.Networks{
			V4: []godo.NetworkV4{
				{
					Type:      "public",
					IPAddress: "1.2.3.4",
				},
			},
		},
	}

	mockDO.On("GetDroplet", mock.Anything, droplet.ID).Return(droplet, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

	container := types.Container{
		ID: "test-container-id",
		Ports: []types.Port{
			{
				PrivatePort: 80,
				PublicPort:  80,
				Type:        "tcp",
			},
		},
	}
	mockDocker.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{container}, nil)
	mockDocker.On("ContainerInspect", mock.Anything, mock.Anything).Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State: &types.ContainerState{
				Status: "running",
			},
		},
	}, nil)
	mockDocker.On("ContainerStart", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	task := &Task{
		state: &TaskState{
			ID:           droplet.ID,
			Name:         "test-task",
			ProviderName: "test-provider",
			Definition: provider.TaskDefinition{
				Name: "test-task",
				Image: provider.ImageDefinition{
					Image: "nginx:latest",
					UID:   "1000",
					GID:   "1000",
				},
				Ports: []string{"80"},
			},
			Status: provider.TASK_STOPPED,
		},
		logger:       logger,
		dockerClient: mockDocker,
		doClient:     mockDO,
	}

	err := task.Start(ctx)
	require.NoError(t, err)
	require.Equal(t, provider.TASK_RUNNING, task.GetState().Status)

	externalAddr, err := task.GetExternalAddress(ctx, "80")
	require.NoError(t, err)
	require.Equal(t, "1.2.3.4:80", externalAddr)

	status, err := task.GetStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, provider.TASK_RUNNING, status)

	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", externalAddr), nil)
	require.NoError(t, err)
	require.NotEmpty(t, req)

	mockDocker.AssertExpectations(t)
	mockDO.AssertExpectations(t)
}

func TestGetStatus(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	tests := []struct {
		name           string
		dropletStatus  string
		containerState string
		setupMocks     func(mockDocker *mocks.DockerClient, mockDO *mocks.DoClient)
		expectedStatus provider.TaskStatus
		expectError    bool
	}{
		{
			name:           "droplet not active",
			dropletStatus:  "off",
			containerState: "",
			setupMocks: func(mockDocker *mocks.DockerClient, mockDO *mocks.DoClient) {
				droplet := &godo.Droplet{
					ID:     123,
					Status: "off",
				}
				mockDO.On("GetDroplet", mock.Anything, droplet.ID).Return(droplet, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)
			},
			expectedStatus: provider.TASK_STOPPED,
			expectError:    false,
		},
		{
			name:           "container running",
			dropletStatus:  "active",
			containerState: "running",
			setupMocks: func(mockDocker *mocks.DockerClient, mockDO *mocks.DoClient) {
				droplet := &godo.Droplet{
					ID:     123,
					Status: "active",
				}
				mockDO.On("GetDroplet", mock.Anything, droplet.ID).Return(droplet, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

				container := types.Container{ID: "test-container-id"}
				mockDocker.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{container}, nil)
				mockDocker.On("ContainerInspect", mock.Anything, container.ID).Return(types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						State: &types.ContainerState{
							Status: "running",
						},
					},
				}, nil)
			},
			expectedStatus: provider.TASK_RUNNING,
			expectError:    false,
		},
		{
			name:           "container paused",
			dropletStatus:  "active",
			containerState: "paused",
			setupMocks: func(mockDocker *mocks.DockerClient, mockDO *mocks.DoClient) {
				droplet := &godo.Droplet{
					ID:     123,
					Status: "active",
				}
				mockDO.On("GetDroplet", mock.Anything, droplet.ID).Return(droplet, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

				container := types.Container{ID: "test-container-id"}
				mockDocker.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{container}, nil)
				mockDocker.On("ContainerInspect", mock.Anything, container.ID).Return(types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						State: &types.ContainerState{
							Status: "paused",
						},
					},
				}, nil)
			},
			expectedStatus: provider.TASK_PAUSED,
			expectError:    false,
		},
		{
			name:           "container stopped state",
			dropletStatus:  "active",
			containerState: "exited",
			setupMocks: func(mockDocker *mocks.DockerClient, mockDO *mocks.DoClient) {
				droplet := &godo.Droplet{
					ID:     123,
					Status: "active",
				}
				mockDO.On("GetDroplet", mock.Anything, droplet.ID).Return(droplet, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

				container := types.Container{ID: "test-container-id"}
				mockDocker.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{container}, nil)
				mockDocker.On("ContainerInspect", mock.Anything, container.ID).Return(types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						State: &types.ContainerState{
							Status: "exited",
						},
					},
				}, nil)
			},
			expectedStatus: provider.TASK_STOPPED,
			expectError:    false,
		},
		{
			name:           "no containers found",
			dropletStatus:  "active",
			containerState: "",
			setupMocks: func(mockDocker *mocks.DockerClient, mockDO *mocks.DoClient) {
				droplet := &godo.Droplet{
					ID:     123,
					Status: "active",
				}
				mockDO.On("GetDroplet", mock.Anything, droplet.ID).Return(droplet, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)
				mockDocker.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{}, nil)
			},
			expectedStatus: provider.TASK_STATUS_UNDEFINED,
			expectError:    true,
		},
		{
			name:           "container inspect error",
			dropletStatus:  "active",
			containerState: "",
			setupMocks: func(mockDocker *mocks.DockerClient, mockDO *mocks.DoClient) {
				droplet := &godo.Droplet{
					ID:     123,
					Status: "active",
				}
				mockDO.On("GetDroplet", mock.Anything, droplet.ID).Return(droplet, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

				container := types.Container{ID: "test-container-id"}
				mockDocker.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{container}, nil)
				mockDocker.On("ContainerInspect", mock.Anything, container.ID).Return(types.ContainerJSON{}, fmt.Errorf("inspect error"))
			},
			expectedStatus: provider.TASK_STATUS_UNDEFINED,
			expectError:    true,
		},
		{
			name:           "container removing",
			dropletStatus:  "active",
			containerState: "removing",
			setupMocks: func(mockDocker *mocks.DockerClient, mockDO *mocks.DoClient) {
				droplet := &godo.Droplet{
					ID:     123,
					Status: "active",
				}
				mockDO.On("GetDroplet", mock.Anything, droplet.ID).Return(droplet, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

				container := types.Container{ID: "test-container-id"}
				mockDocker.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{container}, nil)
				mockDocker.On("ContainerInspect", mock.Anything, container.ID).Return(types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						State: &types.ContainerState{
							Status: "removing",
						},
					},
				}, nil)
			},
			expectedStatus: provider.TASK_STOPPED,
			expectError:    false,
		},
		{
			name:           "container dead",
			dropletStatus:  "active",
			containerState: "dead",
			setupMocks: func(mockDocker *mocks.DockerClient, mockDO *mocks.DoClient) {
				droplet := &godo.Droplet{
					ID:     123,
					Status: "active",
				}
				mockDO.On("GetDroplet", mock.Anything, droplet.ID).Return(droplet, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

				container := types.Container{ID: "test-container-id"}
				mockDocker.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{container}, nil)
				mockDocker.On("ContainerInspect", mock.Anything, container.ID).Return(types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						State: &types.ContainerState{
							Status: "dead",
						},
					},
				}, nil)
			},
			expectedStatus: provider.TASK_STOPPED,
			expectError:    false,
		},
		{
			name:           "container created",
			dropletStatus:  "active",
			containerState: "created",
			setupMocks: func(mockDocker *mocks.DockerClient, mockDO *mocks.DoClient) {
				droplet := &godo.Droplet{
					ID:     123,
					Status: "active",
				}
				mockDO.On("GetDroplet", mock.Anything, droplet.ID).Return(droplet, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

				container := types.Container{ID: "test-container-id"}
				mockDocker.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{container}, nil)
				mockDocker.On("ContainerInspect", mock.Anything, container.ID).Return(types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						State: &types.ContainerState{
							Status: "created",
						},
					},
				}, nil)
			},
			expectedStatus: provider.TASK_STOPPED,
			expectError:    false,
		},
		{
			name:           "unknown container status",
			dropletStatus:  "active",
			containerState: "unknown_status",
			setupMocks: func(mockDocker *mocks.DockerClient, mockDO *mocks.DoClient) {
				droplet := &godo.Droplet{
					ID:     123,
					Status: "active",
				}
				mockDO.On("GetDroplet", mock.Anything, droplet.ID).Return(droplet, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)

				container := types.Container{ID: "test-container-id"}
				mockDocker.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{container}, nil)
				mockDocker.On("ContainerInspect", mock.Anything, container.ID).Return(types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						State: &types.ContainerState{
							Status: "unknown_status",
						},
					},
				}, nil)
			},
			expectedStatus: provider.TASK_STATUS_UNDEFINED,
			expectError:    false,
		},
		{
			name:           "getDroplet error",
			dropletStatus:  "",
			containerState: "",
			setupMocks: func(mockDocker *mocks.DockerClient, mockDO *mocks.DoClient) {
				mockDO.On("GetDroplet", mock.Anything, mock.Anything).Return(nil, nil, fmt.Errorf("failed to get droplet"))
			},
			expectedStatus: provider.TASK_STATUS_UNDEFINED,
			expectError:    true,
		},
		{
			name:           "containerList error",
			dropletStatus:  "active",
			containerState: "",
			setupMocks: func(mockDocker *mocks.DockerClient, mockDO *mocks.DoClient) {
				droplet := &godo.Droplet{
					ID:     123,
					Status: "active",
				}
				mockDO.On("GetDroplet", mock.Anything, droplet.ID).Return(droplet, &godo.Response{Response: &http.Response{StatusCode: http.StatusOK}}, nil)
				mockDocker.On("ContainerList", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("failed to list containers"))
			},
			expectedStatus: provider.TASK_STATUS_UNDEFINED,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDocker := mocks.NewDockerClient(t)
			mockDO := mocks.NewDoClient(t)

			tt.setupMocks(mockDocker, mockDO)

			task := &Task{
				state: &TaskState{
					ID:           123,
					Name:         "test-task",
					ProviderName: "test-provider",
					Definition: provider.TaskDefinition{
						Name: "test-task",
					},
				},
				logger:       logger,
				dockerClient: mockDocker,
				doClient:     mockDO,
			}

			status, err := task.GetStatus(ctx)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedStatus, status)

			mockDocker.AssertExpectations(t)
			mockDO.AssertExpectations(t)
		})
	}
}