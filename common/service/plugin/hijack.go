package plugin

import (
	"bytes"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
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
	out := self.Out()
	return out, nil
}

func (self Hijacked) Out() []byte {
	var b bytes.Buffer
	write := stdcopy.NewStdWriter(&b, stdcopy.Stdout)
	stdcopy.StdCopy(write, nil, self.conn.Reader)
	return b.Bytes()
}

func (self Hijacked) Close() {
	self.conn.Close()
}
