package hostfs

import (
	"errors"
	"os"
	"path/filepath"
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
		if root == "" || strings.IndexByte(root, 0) >= 0 || (root != "/" && (!filepath.IsAbs(root) || filepath.Clean(root) != root)) {
			return errors.New("invalid filesystem root")
		}
		if len(fileSystem.roots) != 0 || fileSystem.remoteFs != nil {
			return errors.New("filesystem backend is already configured")
		}
		if root == "/" {
			roots, err := openSystemRoots()
			if err != nil {
				return err
			}
			fileSystem.roots = roots
			return nil
		}
		localRoot, err := os.OpenRoot(root)
		if err != nil {
			return err
		}
		fileSystem.roots = map[string]*os.Root{"/": localRoot}
		return nil
	}
}

func WithSftpClient(client *sftp.Client) Option {
	return func(fileSystem *Fs) error {
		if client == nil {
			return errors.New("invalid sftp client")
		}
		if len(fileSystem.roots) != 0 || fileSystem.remoteFs != nil {
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
