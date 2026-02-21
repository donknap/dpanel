package acme

import (
	"github.com/donknap/dpanel/common/function"
)

type Option func(self *Acme) error

func WithDomain(list ...string) Option {
	if function.IsEmptyArray(list) {
		return nil
	}
	return func(self *Acme) error {
		for _, d := range list {
			self.argv = append(self.argv, "--domain", d)
		}
		return nil
	}
}

func WithCertServer(server string) Option {
	if server == "" {
		server = "letsencrypt"
	}
	return func(self *Acme) error {
		self.argv = append(self.argv, "--server", server)
		return nil
	}
}

func WithEmail(email string) Option {
	return func(self *Acme) error {
		self.argv = append(self.argv, "--email", email)
		return nil
	}
}

func WithAutoUpgrade(b bool) Option {
	return func(self *Acme) error {
		if b {
			self.argv = append(self.argv, "--auto-upgrade", "1")
		} else {
			self.argv = append(self.argv, "--auto-upgrade", "0")
		}
		return nil
	}
}

func WithForce() Option {
	return func(self *Acme) error {
		self.argv = append(self.argv, "--force")
		return nil
	}
}

func WithDnsNginx() Option {
	return func(self *Acme) error {
		self.argv = append(self.argv, "--webroot", "/var/www/challenges")
		return nil
	}
}

func WithRenew() Option {
	return func(self *Acme) error {
		self.argv = append(self.argv, "--renew")
		return nil
	}
}

func WithIssue() Option {
	return func(self *Acme) error {
		self.argv = append(self.argv, "--issue")
		return nil
	}
}

func WithCertRootPath(path string) Option {
	return func(self *Acme) error {
		self.argv = append(self.argv, "--key-file", path, "--fullchain-file", path)
		return nil
	}
}

func WithDnsApi(apiType string, env []string) Option {
	return func(self *Acme) error {
		self.argv = append(self.argv, "--dns", apiType)
		self.env = append(self.env, env...)
		return nil
	}
}

func WithConfigHomePath(path string) Option {
	return func(self *Acme) error {
		self.argv = append(self.argv, "--config-home", path)
		return nil
	}
}

func WithDebug() Option {
	return func(self *Acme) error {
		self.argv = append(self.argv, "--debug")
		return nil
	}
}

func WithReloadCommand(file string) Option {
	return func(self *Acme) error {
		self.argv = append(self.argv, "--reloadcmd", file)
		return nil
	}
}

func WithEabAccount(kid, hmacKey string) Option {
	return func(self *Acme) error {
		if kid == "" || hmacKey == "" {
			return nil
		}
		self.argv = append(self.argv, "--eab-kid", kid, "--eab-hmac-key", hmacKey)
		return nil
	}
}
