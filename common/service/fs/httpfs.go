package fs

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/donknap/dpanel/common/function"
	"github.com/spf13/afero/mem"
)

func NewHttpFs(fs fs.FS) http.FileSystem {
	return HttpFs{
		fs: http.FS(fs),
	}
}

type HttpFs struct {
	fs http.FileSystem
}

func (self HttpFs) Open(name string) (http.File, error) {
	file, err := self.fs.Open(name)
	if err != nil {
		return file, err
	}
	if strings.HasSuffix(name, ".js") {
		buffer := new(bytes.Buffer)
		_, err = io.Copy(buffer, file)
		if err != nil {
			return nil, err
		}
		content := buffer.String()
		for o, n := range map[string]string{
			"/dpanel":    function.RouterUri("/dpanel"),
			"/ws/common": function.RouterUri("/ws/common"),
			"/api":       function.RouterUri("/api"),
		} {
			content = strings.ReplaceAll(content, o, n)
		}
		memFile := mem.NewFileHandle(mem.CreateFile(name))
		_, err = memFile.WriteString(content)
		if err != nil {
			return nil, err
		}
		_, err = memFile.Seek(0, io.SeekStart)
		if err != nil {
			return nil, err
		}
		return memFile, nil
	}
	return file, nil
}
