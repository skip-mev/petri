package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/volume"
	"github.com/skip-mev/petri/provider"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"
)

// CreateVolume is an idempotent operation
func (p *Provider) CreateVolume(ctx context.Context, definition provider.VolumeDefinition) (string, error) {
	existingVolume, err := p.dockerClient.VolumeInspect(ctx, definition.Name)

	if err == nil {
		return existingVolume.Name, nil
	}

	createdVolume, err := p.dockerClient.VolumeCreate(ctx, volume.CreateOptions{
		Name: definition.Name,
	})

	if err != nil {
		return "", err
	}

	return createdVolume.Name, nil
}

func (p *Provider) DestroyVolume(ctx context.Context, id string) error {
	return p.dockerClient.VolumeRemove(ctx, id, true)
}

// taken from strangelove-ventures/interchain-test
func (p *Provider) WriteFile(ctx context.Context, volumeName, relPath string, content []byte) error {
	const mountPath = "/mnt/dockervolume"

	containerName := fmt.Sprintf("petri-writefile-%d", time.Now().UnixNano())

	cc, err := p.dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: "busybox:latest",

			Entrypoint: []string{"sh", "-c"},
			Cmd: []string{
				// Take the uid and gid of the mount path,
				// and set that as the owner of the new relative path.
				`chown -R "$(stat -c '%u:%g' "$1")" "$2"`,
				"_", // Meaningless arg0 for sh -c with positional args.
				mountPath,
				mountPath,
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

	autoRemoved := false
	defer func() {
		if autoRemoved {
			// No need to attempt removing the container if we successfully started and waited for it to complete.
			return
		}

		if err := p.dockerClient.ContainerRemove(ctx, cc.ID, types.ContainerRemoveOptions{
			Force: true,
		}); err != nil {
			// todo: fix logging

		}
	}()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Name: relPath,

		Size: int64(len(content)),
		Mode: 0600,
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

	if err := p.dockerClient.CopyToContainer(
		ctx,
		cc.ID,
		mountPath,
		&buf,
		types.CopyToContainerOptions{},
	); err != nil {
		return fmt.Errorf("copying tar to container: %w", err)
	}

	if err := p.dockerClient.ContainerStart(ctx, cc.ID, types.ContainerStartOptions{}); err != nil {
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

func (p *Provider) ReadFile(ctx context.Context, volumeName, relPath string) ([]byte, error) {
	const mountPath = "/mnt/dockervolume"

	containerName := fmt.Sprintf("petri-getfile-%d", time.Now().UnixNano())

	cc, err := p.dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: "busybox:latest",

			// Use root user to avoid permission issues when reading files from the volume.
			User: "0",
		},
		&container.HostConfig{
			Binds: []string{volumeName + ":" + mountPath},
			//AutoRemove: true,
		},
		nil, // No networking necessary.
		nil,
		containerName,
	)

	if err != nil {
		return nil, fmt.Errorf("creating container: %w", err)
	}

	//defer func() {
	//	if err := p.dockerClient.ContainerRemove(ctx, cc.ID, types.ContainerRemoveOptions{
	//		Force: true,
	//	}); err != nil {
	//		// todo fix logging
	//	}
	//}()

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
			// todo fix logging
			continue
		}

		return io.ReadAll(tr)
	}

	return nil, fmt.Errorf("path %q not found in tar from container", relPath)
}

func (p *Provider) DownloadDir(ctx context.Context, volumeName, relPath, localPath string) error {
	const mountPath = "/mnt/dockervolume"

	containerName := fmt.Sprintf("petri-getdir-%d", time.Now().UnixNano())

	cc, err := p.dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: "busybox:latest",

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
		if err := p.dockerClient.ContainerRemove(ctx, cc.ID, types.ContainerRemoveOptions{
			Force: true,
		}); err != nil {
			// todo fix logging
		}
	}()

	reader, _, err := p.dockerClient.CopyFromContainer(ctx, cc.ID, relPath)
	if err != nil {
		return err
	}

	if err := os.Mkdir(localPath, os.ModePerm); err != nil {
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
	const mountPath = "/mnt/dockervolume"

	containerName := fmt.Sprintf("petri-setowner-%d", time.Now().UnixNano())

	cc, err := p.dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image:      "busybox:latest",
			Entrypoint: []string{"sh", "-c"},
			Cmd: []string{
				`chown "$2:$3" "$1" && chmod 0700 "$1"`,
				"_", // Meaningless arg0 for sh -c with positional args.
				mountPath,
				uid,
				gid,
			},
			// Use root user to avoid permission issues when reading files from the volume.
			User: "0",
		},
		&container.HostConfig{
			Binds: []string{volumeName + ":" + mountPath},
			//AutoRemove: true,
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

		if err := p.dockerClient.ContainerRemove(ctx, cc.ID, types.ContainerRemoveOptions{
			Force: true,
		}); err != nil {
			// todo fix logging
		}
	}()

	if err := p.dockerClient.ContainerStart(ctx, cc.ID, types.ContainerStartOptions{}); err != nil {
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
