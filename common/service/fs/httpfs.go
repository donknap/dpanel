package fs

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
)

func NewHttpFs(fs fs.FS) http.FileSystem {
	var cacheDir string
	if os.Getenv("APP_ENV") == "debug" {
		cacheDir, _ = storage.Local{}.CreateTempDir("dpanel_js_cache")
	} else {
		cacheDir = filepath.Join(os.TempDir(), "dpanel_js_cache")
	}
	slog.Debug("js cache:", "path", cacheDir)
	_ = os.RemoveAll(cacheDir)
	_ = os.MkdirAll(cacheDir, 0755)

	return HttpFs{
		fs:       http.FS(fs),
		cacheDir: cacheDir,
	}
}

type HttpFs struct {
	fs       http.FileSystem
	cacheDir string
}

func (self HttpFs) Open(name string) (http.File, error) {
	if !strings.HasSuffix(name, ".js") && !strings.HasSuffix(name, ".css") {
		return self.fs.Open(name)
	}

	cleanName := filepath.Clean(name)
	cachePath := filepath.Join(self.cacheDir, cleanName)

	if cachedFile, err := os.Open(cachePath); err == nil {
		return cachedFile, nil
	}

	var content []byte

	origFile, err := self.fs.Open(name)

	if os.IsNotExist(err) || errors.Is(err, fs.ErrNotExist) {
		gzFile, gzErr := self.fs.Open(name + ".gz")
		if gzErr != nil {
			return nil, gzErr
		}
		defer gzFile.Close()

		gzReader, err := gzip.NewReader(gzFile)
		if err != nil {
			return nil, err
		}
		defer gzReader.Close()

		content, err = io.ReadAll(gzReader)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		content, err = io.ReadAll(origFile)
		origFile.Close()
		if err != nil {
			return nil, err
		}
	}

	if facade.Config.GetString("system.baseurl") != "" {
		for _, v := range []string{
			"/dpanel/api", "/dpanel/ws",
			"/dpanel/ui", "/dpanel/static",
		} {
			content = bytes.ReplaceAll(content, []byte(v), []byte(function.RouterUri(v)))
		}
	}

	_ = os.MkdirAll(filepath.Dir(cachePath), 0755)
	tempFile, err := os.CreateTemp(filepath.Dir(cachePath), "tmp_js_*")
	if err != nil {
		return nil, err
	}
	tempName := tempFile.Name()
	_, _ = tempFile.Write(content)
	tempFile.Close()

	_ = os.Rename(tempName, cachePath)

	return os.Open(cachePath)
}
