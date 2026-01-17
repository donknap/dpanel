package types

type DockerInfo struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	KernelVersion   string `json:"kernelVersion"`
	Architecture    string `json:"architecture"`
	OperatingSystem string `json:"operatingSystem"`
	OSType          string `json:"OSType"`
	InDPanel        bool   `json:"inDPanel"` // 是否在 DPanel 环境中，如果在才可以有转发功能
}

type DockerStatus struct {
	Available bool   `json:"available"`
	Message   string `json:"message"`
}
