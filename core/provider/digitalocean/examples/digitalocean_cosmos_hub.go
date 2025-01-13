package main

import (
	"context"
	"os"

	"github.com/skip-mev/petri/core/v2/provider"
	"github.com/skip-mev/petri/core/v2/provider/digitalocean"
	"go.uber.org/zap"
)

func main() {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	doToken := os.Getenv("DO_API_TOKEN")
	if doToken == "" {
		logger.Fatal("DO_API_TOKEN environment variable not set")
	}

	imageID := os.Getenv("DO_IMAGE_ID")
	if imageID == "" {
		logger.Fatal("DO_IMAGE_ID environment variable not set")
	}

	sshKeyPair, err := digitalocean.MakeSSHKeyPair()
	if err != nil {
		logger.Fatal("failed to create SSH key pair", zap.Error(err))
	}

	doProvider, err := digitalocean.NewProvider(ctx, logger, "cosmos-hub", doToken, []string{}, sshKeyPair)
	if err != nil {
		logger.Fatal("failed to create DigitalOcean provider", zap.Error(err))
	}

	taskDef := provider.TaskDefinition{
		Name:          "cosmos-hub-node",
		ContainerName: "cosmos-hub-node",
		Image: provider.ImageDefinition{
			Image: "ghcr.io/cosmos/gaia:v21.0.1",
			UID:   "1000",
			GID:   "1000",
		},
		Ports:       []string{"26656", "26657", "26660", "1317", "9090"},
		Environment: map[string]string{},
		DataDir:     "/root/.gaia",
		Entrypoint:  []string{"/bin/sh", "-c"},
		Command: []string{
			"start",
		},
		ProviderSpecificConfig: digitalocean.DigitalOceanTaskConfig{
			"size":     "s-2vcpu-4gb",
			"region":   "ams3",
			"image_id": imageID,
		},
	}

	logger.Info("Creating new task.")

	task, err := doProvider.CreateTask(ctx, taskDef)
	if err != nil {
		logger.Fatal("failed to create task", zap.Error(err))
	}

	logger.Info("Successfully created task. Starting task now: ")

	err = task.Start(ctx)
	if err != nil {
		logger.Fatal("failed to start task", zap.Error(err))
	}

	logger.Info("Task is successfully running!")
}
