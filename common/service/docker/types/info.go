package types

type DockerInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	KernelVersion string `json:"kernelVersion"`
	Architecture  string `json:"architecture"`
	OSType        string `json:"OSType"`
	InDPanel      bool   `json:"inDPanel"`
}

type DockerStatus struct {
	Available bool   `json:"available"`
	Message   string `json:"message"`
}
