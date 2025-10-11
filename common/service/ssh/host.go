package ssh

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/donknap/dpanel/common/service/exec/local"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var keyErr *knownhosts.KeyError

func NewDefaultKnownHostCallback() *DefaultKnownHostsCallback {
	homeDir, _ := os.UserHomeDir()
	return &DefaultKnownHostsCallback{
		path: filepath.Join(homeDir, ".ssh", "known_hosts"),
	}
}

type DefaultKnownHostsCallback struct {
	path string
}

func (self DefaultKnownHostsCallback) Handler(hostname string, remote net.Addr, key ssh.PublicKey) error {
	var ok bool
	var err error

	// 如果找到了并且有错才表示有问题，否则正常添加 host
	if ok, err = self.check(hostname, remote, key); ok && err != nil {
		return err
	}
	if !ok {
		err = self.add(hostname, remote, key)
		if err != nil {
			return err
		}
	}
	return nil
}

func (self DefaultKnownHostsCallback) check(hostname string, remote net.Addr, key ssh.PublicKey) (found bool, err error) {
	if _, err = os.Stat(self.path); err != nil {
		_, _ = os.Create(self.path)
	}

	callback, err := knownhosts.New(self.path)
	if err != nil {
		return false, err
	}
	err = callback(hostname, remote, key)
	if err == nil {
		return true, nil
	}
	// Make sure that the error returned from the callback is host not in file error.
	// If keyErr.Want is greater than 0 length, that means host is in file with different key.
	if errors.As(err, &keyErr) && len(keyErr.Want) > 0 {
		return true, keyErr
	}
	if err != nil {
		return false, err
	}
	return false, nil
}

func (self DefaultKnownHostsCallback) add(hostname string, remote net.Addr, key ssh.PublicKey) error {
	var err error
	f, err := os.OpenFile(self.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()
	remoteNormalized := knownhosts.Normalize(remote.String())
	hostNormalized := knownhosts.Normalize(hostname)
	addresses := []string{remoteNormalized}

	if hostNormalized != remoteNormalized {
		addresses = append(addresses, hostNormalized)
	}
	_, err = f.WriteString(knownhosts.Line(addresses, key) + "\n")
	return err
}

func (self DefaultKnownHostsCallback) Delete(address string, port int) error {
	host := ""
	if port == 22 {
		host = address
	} else {
		host = fmt.Sprintf("[%s]:%d", address, port)
	}
	_, err := local.QuickRun(fmt.Sprintf(`ssh-keygen -R "%s"`, host))
	return err
}
