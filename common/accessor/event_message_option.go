package accessor

type EventMessageOption struct {
	Content map[string]string `json:"content"`
	Err     string            `json:"err"`
}
