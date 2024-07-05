package plugin

import "log/slog"

func newResult() *result {
	o := &result{
		data: make([]byte, 0),
	}
	return o
}

type result struct {
	data []byte
}

func (self *result) Write(p []byte) (n int, err error) {
	self.data = append(self.data, p...)
	return len(p), nil
}

func (self *result) GetData() []byte {
	slog.Debug("explorer", "copy result", string(self.data))
	return self.data
}
