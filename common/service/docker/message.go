package docker

type progressStatus struct {
	Status string
}

type progressErrDetail struct {
	ErrorDetail struct {
		Message string
	} `json:"errorDetail"`
	Error string `json:"error"`
}

type progressDetail struct {
	Id             string `json:"id"`
	Status         string `json:"status"`
	ProgressDetail struct {
		Current float64 `json:"current"`
		Total   float64 `json:"total"`
	} `json:"progressDetail"`
}

type progressStream struct {
	Stream string `json:"stream"`
	Step   struct {
		Total   string `json:"total"`
		Current string `json:"current"`
	} `json:"step"`
}

type progressAux struct {
	Aux struct {
		ID string
	}
}

type ProgressRemoteImageAux struct {
	Aux struct {
		Tag    string `json:"Tag"`
		Digest string `json:"Digest"`
		Size   int32  `json:"Size"`
	} `json:"aux"`
}

type Progress struct {
	TaskId string // 用于标识任务进度id
	Detail *progressDetail
	Stream *progressStream
	Aux    *progressAux
	Status *progressStatus
	Err    error
}
