package docker

import (
	"bytes"
	"github.com/docker/docker/pkg/stdcopy"
	"io"
)

func GetContentFromStdFormat(reader io.Reader) (*bytes.Buffer, error) {
	buffer := new(bytes.Buffer)
	_, err := io.Copy(buffer, reader)
	if err != nil {
		return nil, err
	}
	newReader := bytes.NewReader(buffer.Bytes())
	stdout := new(bytes.Buffer)
	_, err = stdcopy.StdCopy(stdout, stdout, newReader)
	if err == nil {
		return stdout, nil
	} else {
		return buffer, nil
	}
}
