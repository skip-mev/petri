package util

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
)

// MakeSingleFileTar creates a tar archive of a single file.
// It returns the tar archive as an io.Reader object
func MakeSingleFileTar(name string, file io.Reader) (io.Reader, error) {
	b := bytes.NewBuffer([]byte{})

	tw := tar.NewWriter(b)

	var fileSize int64

	if r, ok := file.(interface{ Size() int64 }); ok {
		fileSize = r.Size()
	} else {
		return nil, errors.New("unable to determine file size")
	}

	header := &tar.Header{
		Name: name,
		Size: fileSize,
		Mode: 0777,
	}

	if err := tw.WriteHeader(header); err != nil {
		return nil, err
	}

	_, err := io.Copy(tw, file)

	if err != nil {
		return nil, err
	}

	return b, nil
}

func UnarchiveSingleFileTar(archive io.Reader) (string, io.Reader, error) {
	tarReader := tar.NewReader(archive)

	for true {
		header, err := tarReader.Next()

		if err == io.EOF {
			return "", nil, fmt.Errorf("no files found in tar archive")
		}

		if err != nil {
			return "", nil, fmt.Errorf("extract failed: %v", err)
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		b := bytes.NewBuffer([]byte{})

		_, err = io.Copy(b, tarReader)

		if err != nil {
			return "", nil, fmt.Errorf("extract failed: %v", err)
		}

		return header.Name, b, nil
	}

	return "", nil, fmt.Errorf("no files found in tar archive")
}
