package hostfs

import (
	"errors"
	"os"
	"path"
	"strings"

	"github.com/pkg/sftp"
	aferosftpfs "github.com/spf13/afero/sftpfs"
)

type Option func(*Fs) error

func WithName(name string) Option {
	return func(fileSystem *Fs) error {
		fileSystem.name = name
		return nil
	}
}

func WithRoot(root string) Option {
	return func(fileSystem *Fs) error {
		if root == "" || strings.IndexByte(root, 0) >= 0 || !path.IsAbs(root) || path.Clean(root) != root {
			return errors.New("invalid filesystem root")
		}
		localRoot, err := os.OpenRoot(root)
		if err != nil {
			return err
		}
		if fileSystem.root != nil || fileSystem.remoteFs != nil {
			_ = localRoot.Close()
			return errors.New("filesystem backend is already configured")
		}
		fileSystem.root = localRoot
		return nil
	}
}

func WithSftpClient(client *sftp.Client) Option {
	return func(fileSystem *Fs) error {
		if client == nil {
			return errors.New("invalid sftp client")
		}
		if fileSystem.root != nil || fileSystem.remoteFs != nil {
			return errors.New("filesystem backend is already configured")
		}
		fileSystem.sftpClient = client
		fileSystem.remoteFs = aferosftpfs.New(client)
		return nil
	}
}

func WithWorkingDir(workingDir string) Option {
	return func(fileSystem *Fs) error {
		fileSystem.workingDir = workingDir
		return nil
	}
}
