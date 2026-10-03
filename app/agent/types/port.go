package types

type PortCheckItem struct {
	Port          string   `json:"port"`
	Status        string   `json:"status"`
	LatencyMillis *float64 `json:"latencyMillis,omitempty"`
	Error         string   `json:"error,omitempty"`
}

type PortCheckError struct {
	Check  string `json:"check"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

type PortCheckResult struct {
	ContainerID string           `json:"containerId"`
	Ports       []PortCheckItem  `json:"ports"`
	Errors      []PortCheckError `json:"errors,omitempty"`
}
