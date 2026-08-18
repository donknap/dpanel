package types

import "strings"

// Registry describes a source registry and its pull mirrors.
type Registry struct {
	Address      string
	ProxyAddress []string
	EnableHttp   bool
	Auth         string
}

// APIAddresses returns proxy API endpoints and the source registry API endpoint.
// The returned values are for registry HTTP clients; Docker image references
// must normalize them to a host without a URL scheme before use.
func (self Registry) APIAddresses() ([]string, string) {
	proxyAddresses := append([]string(nil), self.ProxyAddress...)
	sourceAddress := strings.TrimSpace(self.Address)
	if sourceAddress != "" && self.EnableHttp && !strings.Contains(sourceAddress, "://") {
		sourceAddress = "http://" + sourceAddress
	}
	return proxyAddresses, sourceAddress
}
