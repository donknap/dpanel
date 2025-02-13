package acme

import (
	"github.com/donknap/dpanel/common/accessor"
	"github.com/donknap/dpanel/common/function"
)

type Option func() []string

func WithDomainList(list ...string) Option {
	if function.IsEmptyArray(list) {
		return nil
	}
	return func() []string {
		domainList := make([]string, 0)
		for _, d := range list {
			domainList = append(domainList, "--domain", d)
		}
		return domainList
	}
}

func WithDomain(domain string) Option {
	return func() []string {
		return []string{"--domain", domain}
	}
}

func WithCertServer(server string) Option {
	if server == "" {
		server = "letsencrypt"
	}
	return func() []string {
		return []string{"--server", server}
	}
}

func WithEmail(email string) Option {
	return func() []string {
		return []string{"--email", email}
	}
}

func WithAutoUpgrade() Option {
	return func() []string {
		return []string{"--auto-upgrade", "1"}
	}
}

func WithForce() Option {
	return func() []string {
		return []string{"--force"}
	}
}

func WithDnsNginx() Option {
	return func() []string {
		return []string{"--nginx"}
	}
}

func WithRenew() Option {
	return func() []string {
		return []string{"--renew"}
	}
}

func WithIssue() Option {
	return func() []string {
		return []string{"--issue"}
	}
}

func WithCertRootPath(path string) Option {
	return func() []string {
		return []string{"--key-file", path, "--fullchain-file", path}
	}
}

func WithDnsApi(api accessor.DnsApi) Option {
	return func() []string {
		return []string{"--dns", api.ServerName}
	}
}

func WithConfigHomePath(path string) Option {
	return func() []string {
		return []string{"--config-home", path}
	}
}

func WithDebug() Option {
	return func() []string {
		return []string{"--debug"}
	}
}
