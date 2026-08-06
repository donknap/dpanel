package function

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	SSRFAllowLoopback = 1 << iota
	SSRFAllowUnspecified
	SSRFAllowLinkLocal
	SSRFAllowPrivate
)

var ssrfBlockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
}

type safeHTTPAddressesKey struct{}

type safeHTTPRoundTripper struct {
	direct *http.Transport
	proxy  *http.Transport
}

func (self safeHTTPRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	proxyURL, err := http.ProxyFromEnvironment(request)
	if err != nil {
		return nil, err
	}
	if proxyURL != nil {
		// 面板代理由管理员显式配置，代理链路保留 URL 和重定向安全校验，目标解析交由可信代理完成。
		return self.proxy.RoundTrip(request)
	}
	return self.direct.RoundTrip(request)
}

var safeHTTPTransport = func() http.RoundTripper {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return dialSafeHTTPAddresses(ctx, network, address, (&net.Dialer{}).DialContext)
	}
	proxyTransport := http.DefaultTransport.(*http.Transport).Clone()
	return safeHTTPRoundTripper{
		direct: transport,
		proxy:  proxyTransport,
	}
}()

func dialSafeHTTPAddresses(ctx context.Context, network, address string, dialContext func(context.Context, string, string) (net.Conn, error)) (net.Conn, error) {
	addresses, ok := ctx.Value(safeHTTPAddressesKey{}).([]net.IP)
	if !ok || len(addresses) == 0 {
		return nil, errors.New("validated http address is unavailable")
	}
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	var dialErrors []error
	for _, ip := range addresses {
		connection, err := dialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return connection, nil
		}
		dialErrors = append(dialErrors, err)
	}
	return nil, errors.Join(dialErrors...)
}

func IpInSubnet(ipAddress, subnetAddress string) (bool, error) {
	ip := net.ParseIP(ipAddress)
	if ip == nil {
		return false, errors.New("ip address incorrect: " + ipAddress)
	}
	_, subnet, err := net.ParseCIDR(subnetAddress)
	if err != nil {
		return false, errors.New("CIDR address incorrect: " + subnetAddress)
	}

	if subnetAddress != subnet.String() {
		return false, errors.New("CIDR address incorrect, like: " + subnet.String())
	}
	if !subnet.Contains(ip) {
		return false, errors.New("ip address does not match the subnet address")
	}
	return true, nil
}

func SafeHTTPAddresses(rawURL string, flags ...int) ([]net.IP, error) {
	uri, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if uri.Scheme != "http" && uri.Scheme != "https" {
		return nil, errors.New("unsupported url scheme")
	}
	host := uri.Hostname()
	if host == "" {
		return nil, errors.New("invalid url host")
	}
	flagValue := 0
	for _, flag := range flags {
		flagValue |= flag
	}
	if strings.EqualFold(host, "localhost") && flagValue&SSRFAllowLoopback == 0 {
		return nil, errors.New("localhost is not allowed")
	}
	ips, err := net.DefaultResolver.LookupIP(context.Background(), "ip", host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, errors.New("url host has no ip address")
	}
	for _, ip := range ips {
		if ip.IsLoopback() && flagValue&SSRFAllowLoopback == 0 {
			return nil, errors.New("loopback address is not allowed")
		}
		if ip.IsUnspecified() && flagValue&SSRFAllowUnspecified == 0 {
			return nil, errors.New("unspecified address is not allowed")
		}
		if (ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()) && flagValue&SSRFAllowLinkLocal == 0 {
			return nil, errors.New("link-local address is not allowed")
		}
		if ip.IsPrivate() && flagValue&SSRFAllowPrivate == 0 {
			return nil, errors.New("private address is not allowed")
		}
		address, ok := netip.AddrFromSlice(ip)
		if !ok {
			return nil, errors.New("invalid resolved ip address")
		}
		address = address.Unmap()
		for _, prefix := range ssrfBlockedPrefixes {
			if prefix.Contains(address) {
				return nil, errors.New("special-purpose address is not allowed")
			}
		}
	}
	return ips, nil
}

// SafeHTTPGet 获取远程 HTTP 数据，并统一限制内网地址、重定向、超时和响应体大小。
func SafeHTTPGet(ctx context.Context, rawURL string, timeout time.Duration, maxBodySize int64) (*http.Response, error) {
	if ctx == nil {
		return nil, errors.New("http context is nil")
	}
	if timeout <= 0 {
		return nil, errors.New("http timeout must be greater than zero")
	}
	if maxBodySize <= 0 {
		return nil, errors.New("http max body size must be greater than zero")
	}
	addresses, err := SafeHTTPAddresses(rawURL)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: safeHTTPTransport,
		Timeout:   timeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			redirectAddresses, err := SafeHTTPAddresses(request.URL.String())
			if err != nil {
				return err
			}
			*request = *request.WithContext(context.WithValue(request.Context(), safeHTTPAddressesKey{}, redirectAddresses))
			return nil
		},
	}
	request, err := http.NewRequestWithContext(context.WithValue(ctx, safeHTTPAddressesKey{}, addresses), http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.ContentLength > maxBodySize {
		_ = response.Body.Close()
		return nil, fmt.Errorf("http response body exceeds %d bytes", maxBodySize)
	}
	response.Body = http.MaxBytesReader(nil, response.Body, maxBodySize)
	return response, nil
}

func ValidateDomainName(domain string) error {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return errors.New("domain is empty")
	}
	if net.ParseIP(domain) != nil {
		return errors.New("domain cannot be ip")
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return errors.New("domain cannot start or end with dot")
	}
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return errors.New("domain must include at least one dot")
	}
	for _, label := range parts {
		if label == "" || len(label) > 63 {
			return errors.New("domain label length is invalid")
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return errors.New("domain label cannot start or end with hyphen")
		}
		for _, ch := range label {
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' {
				continue
			}
			return errors.New("domain contains invalid character")
		}
	}
	return nil
}

func SystemResolver(defaultDnsIps ...string) []string {
	resolvers := make([]string, 0, 3)
	file, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return defaultDnsIps
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "nameserver") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		rawIp := fields[1]
		ip := net.ParseIP(rawIp)
		if ip == nil {
			continue
		}
		if ipv4 := ip.To4(); ipv4 != nil {
			ipStr := ipv4.String()
			resolvers = append(resolvers, ipStr)
		}
	}
	if len(resolvers) == 0 {
		return defaultDnsIps
	}
	return resolvers
}
