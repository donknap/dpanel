package function

import (
	"bufio"
	"errors"
	"net"
	"os"
	"strings"
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

func SystemResolver(defaultDnsIps ...string) []string {
	var resolvers = make([]string, 0)
	file, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return defaultDnsIps
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "nameserver") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				resolvers = append(resolvers, fields[1])
			}
		}
	}
	if len(resolvers) == 0 {
		return defaultDnsIps
	}
	return resolvers
}
