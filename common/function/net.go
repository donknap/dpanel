package function

import (
	"errors"
	"net"
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
