package common

type CacheOption struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Keep  int    `json:"keep"`
}

type CacheResult struct {
	Value string `json:"value"`
	Found bool   `json:"found"`
}

type NotificationOption struct {
	Channel string `json:"channel"`
	Subject string `json:"subject"`
	Content string `json:"content"`
	Target  string `json:"target"`
}
