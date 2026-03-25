package docker

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	ioFs "io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/docker/docker/client"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/types/fs"
)

func (self Client) ImageInspectFileList(ctx context.Context, imageID string) (pathInfo []*fs.FileData, pathList []string, err error) {
	_, err = self.Client.ImageInspect(ctx, imageID)
	if err != nil {
		return nil, nil, err
	}
	out, err := self.Client.ImageSave(ctx, []string{
		imageID,
	})
	if err != nil {
		return nil, nil, err
	}
	defer out.Close()

	tarReader := tar.NewReader(out)
	for {
		header, err := tarReader.Next()
		if err != nil {
			break
		}
		if header.FileInfo().IsDir() {
			continue
		}

		name := header.Name
		isPossibleLayer := strings.HasSuffix(name, ".tar") ||
			strings.HasSuffix(name, ".tar.gz") ||
			strings.HasSuffix(name, ".tgz") ||
			strings.HasPrefix(name, "blobs/") ||
			strings.Contains(name, "/layer.tar")

		if !isPossibleLayer {
			continue
		}

		bufReader := bufio.NewReader(tarReader)
		var layerReader io.Reader = bufReader
		var gzReader *gzip.Reader

		magic, err := bufReader.Peek(2)
		if err == nil && len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
			gzReader, err = gzip.NewReader(bufReader)
			if err == nil {
				layerReader = gzReader
			}
		}

		layerTar := tar.NewReader(layerReader)
		tarFileList, err := getFileListFromTar(layerTar)

		if gzReader != nil {
			_ = gzReader.Close()
		}

		if err != nil {
			slog.Debug("docker image inspect file list: skip non-tar layer", "name", name, "error", err)
			continue
		}
		pathInfo = append(pathInfo, tarFileList...)
	}
	sort.Slice(pathInfo, func(i, j int) bool {
		return pathInfo[i].IsDir && !pathInfo[j].IsDir
	})
	sort.Slice(pathInfo, func(i, j int) bool {
		if pathInfo[i].IsDir != pathInfo[j].IsDir {
			return pathInfo[i].IsDir
		}
		return pathInfo[i].Name < pathInfo[j].Name
	})
	pathList = make([]string, 0)
	pathInfo = function.PluckArrayWalk(pathInfo, func(i *fs.FileData) (*fs.FileData, bool) {
		if function.InArray(pathList, i.Name) {
			return nil, false
		} else {
			pathList = append(pathList, i.Name)
			return i, true
		}
	})
	return pathInfo, pathList, nil
}

func getFileListFromTar(tarReader *tar.Reader) (files []*fs.FileData, err error) {
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		// always ensure relative path notations are not parsed as part of the filename
		name := path.Clean(header.Name)
		if name == "." {
			continue
		}

		switch header.Typeflag {
		case tar.TypeXGlobalHeader:
			return nil, fmt.Errorf("unexptected tar file: (XGlobalHeader): type=%v name=%s", header.Typeflag, name)
		case tar.TypeXHeader:
			return nil, fmt.Errorf("unexptected tar file (XHeader): type=%v name=%s", header.Typeflag, name)
		default:
			files = append(files, &fs.FileData{
				Path:      filepath.Join("/", header.Name),
				Name:      filepath.Join("/", header.Name),
				Mod:       os.FileMode(header.Mode),
				ModStr:    os.FileMode(header.Mode).String(),
				ModTime:   header.ModTime,
				Change:    fs.ChangeDefault,
				Size:      header.Size,
				User:      fmt.Sprintf("%d", header.Uid),
				Group:     fmt.Sprintf("%d", header.Gid),
				LinkName:  header.Linkname,
				IsDir:     header.Typeflag == tar.TypeDir,
				IsSymlink: false,
			})
		}
	}
	return files, nil
}

func (self Client) ImageLoadFsFile(ctx context.Context, file ioFs.File) error {
	reader, err := self.Client.ImageLoad(ctx, file, client.ImageLoadWithQuiet(false))
	if err != nil {
		return err
	}
	defer reader.Body.Close()

	_, err = io.Copy(io.Discard, reader.Body)
	if err != nil {
		return err
	}
	return nil
}
