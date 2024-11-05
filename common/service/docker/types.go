package docker

type PullMessage struct {
	Id             string `json:"id"`
	Status         string `json:"status"`
	Progress       string `json:"progress"`
	ProgressDetail struct {
		Current float64 `json:"current"`
		Total   float64 `json:"total"`
	} `json:"progressDetail"`
}

type BuildMessage struct {
	Stream      string `json:"stream"`
	ErrorDetail struct {
		Message string `json:"message"`
	} `json:"errorDetail"`
	PullMessage
}

type PullProgress struct {
	Downloading float64 `json:"downloading"`
	Extracting  float64 `json:"extracting"`
}
