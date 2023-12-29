package plugin

import (
	"github.com/docker/docker/api/types"
)

type Hijacked struct {
	conn types.HijackedResponse
	Id   string
}

func (self Hijacked) Run(cmd string) ([]byte, error) {
	_, err := self.conn.Conn.Write([]byte(cmd))
	if err != nil {
		return nil, err
	}
	bufLen := 256
	var out []byte
	for {
		buf := make([]byte, bufLen)
		n, err := self.conn.Conn.Read(buf)
		if err != nil {
			break
		}
		if n < bufLen {
			out = append(out, buf[0:n]...)
			break
		}
		out = append(out, buf...)
	}
	return out, nil
}
