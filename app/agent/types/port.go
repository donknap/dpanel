package types

type Port struct {
	Port     uint16 `json:"port"`
	Protocol string `json:"protocol"`
}

type PortResult struct {
	ContainerID string `json:"containerId"`
	Ports       []Port `json:"ports"`
	Error       string `json:"error,omitempty"`
}
