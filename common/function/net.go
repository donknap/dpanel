package function

import (
	"bufio"
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
)

const (
	SSRFAllowLoopback = 1 << iota
	SSRFAllowUnspecified
	SSRFAllowLinkLocal
	SSRFAllowPrivate
)

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

func IpIsLocalhost(address string) bool {
	host := address
	if h, _, err := net.SplitHostPort(address); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if strings.ToLower(host) == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func CheckSSRFURL(raw string, flags ...int) error {
	uri, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if uri.Scheme != "http" && uri.Scheme != "https" {
		return errors.New("unsupported url scheme")
	}
	host := uri.Hostname()
	if host == "" {
		return errors.New("invalid url host")
	}
	flagValue := 0
	for _, flag := range flags {
		flagValue |= flag
	}
	if strings.EqualFold(host, "localhost") && flagValue&SSRFAllowLoopback == 0 {
		return errors.New("localhost is not allowed")
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		if ip.IsLoopback() && flagValue&SSRFAllowLoopback == 0 {
			return errors.New("loopback address is not allowed")
		}
		if ip.IsUnspecified() && flagValue&SSRFAllowUnspecified == 0 {
			return errors.New("unspecified address is not allowed")
		}
		if (ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()) && flagValue&SSRFAllowLinkLocal == 0 {
			return errors.New("link-local address is not allowed")
		}
		if ip.IsPrivate() && flagValue&SSRFAllowPrivate == 0 {
			return errors.New("private address is not allowed")
		}
	}
	return nil
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
