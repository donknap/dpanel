package agent

type ContainerTarget struct {
	ContainerID string
	PID         int
}

type PortCheckTarget struct {
	ContainerID string   `json:"containerId"`
	Ports       []string `json:"ports"`
}
