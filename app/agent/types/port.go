package types

type PortCheckItem struct {
	Port          string   `json:"port"`
	Status        string   `json:"status"`
	LatencyMillis *float64 `json:"latencyMillis,omitempty"`
}

type PortCheckResult struct {
	ContainerID string          `json:"containerId"`
	Ports       []PortCheckItem `json:"ports"`
}
