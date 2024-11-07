package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"go.uber.org/zap"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"

	"github.com/skip-mev/petri/core/v2/provider"
)

// CreateVolume is an idempotent operation
func (p *Provider) CreateVolume(ctx context.Context, definition provider.VolumeDefinition) (string, error) {
	if err := definition.ValidateBasic(); err != nil {
		return "", fmt.Errorf("failed to validate volume definition: %w", err)
	}

	p.logger.Debug("creating volume", zap.String("name", definition.Name), zap.String("size", definition.Size))

	existingVolume, err := p.dockerClient.VolumeInspect(ctx, definition.Name)

	if err == nil {
		return existingVolume.Name, nil
	}

	createdVolume, err := p.dockerClient.VolumeCreate(ctx, volume.CreateOptions{
		Name: definition.Name,
		Labels: map[string]string{
			providerLabelName: p.name,
		},
	})
	if err != nil {
		return "", err
	}

	return createdVolume.Name, nil
}

func (p *Provider) DestroyVolume(ctx context.Context, id string) error {
	p.logger.Info("destroying volume", zap.String("id", id))

	return p.dockerClient.VolumeRemove(ctx, id, true)
}

// taken from strangelove-ventures/interchain-test
func (p *Provider) WriteFile(ctx context.Context, id, relPath string, content []byte) error {
	dockerContainer, err := p.dockerClient.ContainerInspect(ctx, id)
	if err != nil {
		return err
	}

	if len(dockerContainer.Mounts) == 0 {
		return fmt.Errorf("no volumes found for container %s", id)
	}

	volumeName := dockerContainer.Mounts[0].Name

	logger := p.logger.With(zap.String("volume", id), zap.String("path", relPath))

	logger.Debug("writing file")

	const mountPath = "/mnt/dockervolume"

	containerName := fmt.Sprintf("petri-writefile-%d", time.Now().UnixNano())

	if err := p.pullImage(ctx, p.builderImageName); err != nil {
		return err
	}

	logger.Debug("creating writefile container")

	cc, err := p.dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: p.builderImageName,

			Entrypoint: []string{"sh", "-c"},
			Cmd: []string{
				// Take the uid and gid of the mount path,
				// and set that as the owner of the new relative path.
				`chown -R "$(stat -c '%u:%g' "$1")" "$2"`,
				"_", // Meaningless arg0 for sh -c with positional args.
				mountPath,
				mountPath,
			},

			Labels: map[string]string{
				providerLabelName: p.name,
			},

			// Use root user to avoid permission issues when reading files from the volume.
			User: "0:0",
		},
		&container.HostConfig{
			Binds:      []string{volumeName + ":" + mountPath},
			AutoRemove: true,
		},
		nil, // No networking necessary.
		nil,
		containerName,
	)
	if err != nil {
		return fmt.Errorf("creating container: %w", err)
	}

	logger.Debug("created writefile container", zap.String("id", cc.ID))

	autoRemoved := false
	defer func() {
		if autoRemoved {
			// No need to attempt removing the container if we successfully started and waited for it to complete.
			return
		}

		if _, err := p.dockerClient.ContainerInspect(ctx, cc.ID); err != nil && client.IsErrNotFound(err) {
			// auto-removed, but not detected as autoremoved
			return
		}

		if err := p.dockerClient.ContainerRemove(ctx, cc.ID, container.RemoveOptions{
			Force: true,
		}); err != nil {
			logger.Error("failed to remove writefile container", zap.String("id", cc.ID), zap.Error(err))
		}
	}()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Name: relPath,

		Size: int64(len(content)),
		Mode: 0o600,
		// Not setting uname because the container will chown it anyway.
		ModTime: time.Now(),

		Format: tar.FormatPAX,
	}); err != nil {
		return fmt.Errorf("writing tar header: %w", err)
	}
	if _, err := tw.Write(content); err != nil {
		return fmt.Errorf("writing content to tar: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("closing tar writer: %w", err)
	}

	logger.Debug("copying file to container")

	if err := p.dockerClient.CopyToContainer(
		ctx,
		cc.ID,
		mountPath,
		&buf,
		container.CopyToContainerOptions{},
	); err != nil {
		return fmt.Errorf("copying tar to container: %w", err)
	}

	logger.Debug("starting writefile container")
	if err := p.dockerClient.ContainerStart(ctx, cc.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("starting write-file container: %w", err)
	}

	waitCh, errCh := p.dockerClient.ContainerWait(ctx, cc.ID, container.WaitConditionNotRunning)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	case res := <-waitCh:
		autoRemoved = true

		if res.Error != nil {
			return fmt.Errorf("waiting for write-file container: %s", res.Error.Message)
		}

		if res.StatusCode != 0 {
			return fmt.Errorf("chown on new file exited %d", res.StatusCode)
		}
	}

	return nil
}

func (p *Provider) ReadFile(ctx context.Context, id, relPath string) ([]byte, error) {
	dockerContainer, err := p.dockerClient.ContainerInspect(ctx, id)
	if err != nil {
		return nil, err
	}

	if len(dockerContainer.Mounts) == 0 {
		return nil, fmt.Errorf("no volumes found for container %s", id)
	}

	volumeName := dockerContainer.Mounts[0].Name

	logger := p.logger.With(zap.String("volume", volumeName), zap.String("path", relPath))

	const mountPath = "/mnt/dockervolume"

	containerName := fmt.Sprintf("petri-getfile-%d", time.Now().UnixNano())

	if err := p.pullImage(ctx, p.builderImageName); err != nil {
		return nil, err
	}

	logger.Debug("creating getfile container")

	cc, err := p.dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: p.builderImageName,

			Labels: map[string]string{
				providerLabelName: p.name,
			},

			// Use root user to avoid permission issues when reading files from the volume.
			User: "0",
		},
		&container.HostConfig{
			Binds: []string{volumeName + ":" + mountPath},
			// AutoRemove: true,
		},
		nil, // No networking necessary.
		nil,
		containerName,
	)
	if err != nil {
		return nil, fmt.Errorf("creating container: %w", err)
	}

	logger.Debug("created getfile container", zap.String("id", cc.ID))

	defer func() {
		if _, err := p.dockerClient.ContainerInspect(ctx, cc.ID); err != nil && client.IsErrNotFound(err) {
			// auto-removed, but not detected as autoremoved
			return
		}

		if err := p.dockerClient.ContainerRemove(ctx, cc.ID, container.RemoveOptions{
			Force: true,
		}); err != nil {
			logger.Error("failed cleaning up the getfile container", zap.Error(err))
		}
	}()

	logger.Debug("copying from container")
	rc, _, err := p.dockerClient.CopyFromContainer(ctx, cc.ID, path.Join(mountPath, relPath))
	if err != nil {
		return nil, fmt.Errorf("copying from container: %w", err)
	}
	defer func() {
		_ = rc.Close()
	}()

	wantPath := path.Base(relPath)
	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading tar from container: %w", err)
		}
		if hdr.Name != wantPath {
			continue
		}

		return io.ReadAll(tr)
	}

	return nil, fmt.Errorf("path %q not found in tar from container", relPath)
}

func (p *Provider) DownloadDir(ctx context.Context, id, relPath, localPath string) error {
	dockerContainer, err := p.dockerClient.ContainerInspect(ctx, id)
	if err != nil {
		return err
	}

	if len(dockerContainer.Mounts) == 0 {
		return fmt.Errorf("no volumes found for container %s", id)
	}

	volumeName := dockerContainer.Mounts[0].Name

	logger := p.logger.With(zap.String("volume", volumeName), zap.String("path", relPath), zap.String("localPath", localPath))

	const mountPath = "/mnt/dockervolume"

	containerName := fmt.Sprintf("petri-getdir-%d", time.Now().UnixNano())

	logger.Debug("creating getdir container")

	if err := p.pullImage(ctx, p.builderImageName); err != nil {
		return err
	}

	cc, err := p.dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: p.builderImageName,

			Labels: map[string]string{
				providerLabelName: p.name,
			},

			// Use root user to avoid permission issues when reading files from the volume.
			User: "0",
		},
		&container.HostConfig{
			Binds:      []string{volumeName + ":" + mountPath},
			AutoRemove: true,
		},
		nil, // No networking necessary.
		nil,
		containerName,
	)
	if err != nil {
		return fmt.Errorf("creating container: %w", err)
	}

	defer func() {
		if _, err := p.dockerClient.ContainerInspect(ctx, cc.ID); err != nil && client.IsErrNotFound(err) {
			return
		}

		if err := p.dockerClient.ContainerRemove(ctx, cc.ID, container.RemoveOptions{
			Force: true,
		}); err != nil {
			logger.Error("failed cleaning up the getdir container", zap.Error(err))
		}
	}()

	logger.Debug("copying from container")
	reader, _, err := p.dockerClient.CopyFromContainer(ctx, cc.ID, path.Join(mountPath, relPath))
	if err != nil {
		return err
	}

	if err := os.MkdirAll(localPath, os.ModePerm); err != nil {
		return err
	}
	tr := tar.NewReader(reader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return err
		}

		var fileBuff bytes.Buffer
		if _, err := io.Copy(&fileBuff, tr); err != nil {
			return err
		}

		name := hdr.Name
		extractedFileName := path.Base(name)
		isDirectory := extractedFileName == ""
		if isDirectory {
			continue
		}

		filePath := filepath.Join(localPath, extractedFileName)
		if err := os.WriteFile(filePath, fileBuff.Bytes(), os.ModePerm); err != nil {
			return err
		}
	}

	return nil
}

func (p *Provider) SetVolumeOwner(ctx context.Context, volumeName, uid, gid string) error {
	logger := p.logger.With(zap.String("volume", volumeName), zap.String("uid", uid), zap.String("gid", gid))

	const mountPath = "/mnt/dockervolume"

	containerName := fmt.Sprintf("petri-setowner-%d", time.Now().UnixNano())

	if err := p.pullImage(ctx, p.builderImageName); err != nil {
		return err
	}

	logger.Debug("creating volume-owner container")

	cc, err := p.dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image:      p.builderImageName,
			Entrypoint: []string{"sh", "-c"},
			Cmd: []string{
				`chown "$2:$3" "$1" && chmod 0700 "$1"`,
				"_", // Meaningless arg0 for sh -c with positional args.
				mountPath,
				uid,
				gid,
			},
			Labels: map[string]string{
				providerLabelName: p.name,
			},
			// Use root user to avoid permission issues when reading files from the volume.
			User: "0",
		},
		&container.HostConfig{
			Binds:      []string{volumeName + ":" + mountPath},
			AutoRemove: true,
		},
		nil, // No networking necessary.
		nil,
		containerName,
	)
	if err != nil {
		return fmt.Errorf("creating container: %w", err)
	}
	autoRemoved := false
	defer func() {
		if autoRemoved {
			// No need to attempt removing the container if we successfully started and waited for it to complete.
			return
		}
    
		if _, err := p.dockerClient.ContainerInspect(ctx, cc.ID); err != nil && client.IsErrNotFound(err) {
			// auto-removed, but not detected as autoremoved
			return
		}

		if err := p.dockerClient.ContainerRemove(ctx, cc.ID, container.RemoveOptions{
			Force: true,
		}); err != nil {
			logger.Error("failed cleaning up the volume-owner container", zap.Error(err))
		}
	}()

	logger.Debug("starting volume-owner container")
	if err := p.dockerClient.ContainerStart(ctx, cc.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("starting volume-owner container: %w", err)
	}

	waitCh, errCh := p.dockerClient.ContainerWait(ctx, cc.ID, container.WaitConditionNotRunning)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	case res := <-waitCh:
		autoRemoved = true

		if res.Error != nil {
			return fmt.Errorf("waiting for volume-owner container: %s", res.Error.Message)
		}

		if res.StatusCode != 0 {
			return fmt.Errorf("configuring volume exited %d", res.StatusCode)
		}
	}

	return nil
}

func (p *Provider) teardownVolumes(ctx context.Context) error {
	p.logger.Debug("tearing down docker volumes")

	volumes, err := p.dockerClient.VolumeList(ctx, volume.ListOptions{
		Filters: filters.NewArgs(filters.Arg("label", fmt.Sprintf("%s=%s", providerLabelName, p.name))),
	})
	if err != nil {
		return err
	}

	for _, filteredVolume := range volumes.Volumes {
		err := p.DestroyVolume(ctx, filteredVolume.Name)
		if err != nil {
			return err
		}
	}

	return nil
}
