package docker

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"github.com/docker/go-units"
	"github.com/donknap/dpanel/common/function"
	"github.com/mcuadros/go-version"
	"io"
	"log/slog"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

func (self Builder) ImageInspectFileList(imageID string) (pathInfo []*FileItemResult, path []string, err error) {
	imageInfo, err := self.Client.ImageInspect(self.Ctx, imageID)
	if err != nil {
		return nil, nil, err
	}
	dockerVersion, _ := self.Client.ServerVersion(self.Ctx)
	// 如果当前 docker 版本大于 25 则获取 rootfs 否则直接查找 tar 的文件
	layers := function.PluckArrayWalk(imageInfo.RootFS.Layers, func(i string) (string, bool) {
		if _, after, ok := strings.Cut(i, "sha256:"); ok {
			return fmt.Sprintf("blobs/sha256/%s", after), true
		}
		return "", false
	})
	out, err := self.Client.ImageSave(self.Ctx, []string{
		imageID,
	})

	tarReader := tar.NewReader(out)
	for {
		header, err := tarReader.Next()
		if err != nil {
			break
		}
		var tarFileList []*FileItemResult
		if version.Compare(dockerVersion.Version, "25", ">=") && function.InArray(layers, header.Name) {
			tarFileList, err = getFileListFromTar(tar.NewReader(tarReader))
			if err != nil {
				slog.Debug("docker image inspect file list", "error", err)
				continue
			}
		} else if strings.HasSuffix(header.Name, ".tar") {
			tarFileList, err = getFileListFromTar(tar.NewReader(tarReader))
			if err != nil {
				slog.Debug("docker image inspect file list", "error", err)
				continue
			}
		} else if strings.HasSuffix(header.Name, ".tar.gz") || strings.HasSuffix(header.Name, "tgz") {
			gzReader, err := gzip.NewReader(tarReader)
			if err != nil {
				return nil, nil, err
			}
			tarFileList, err = getFileListFromTar(tar.NewReader(gzReader))
			_ = gzReader.Close()
			if err != nil {
				slog.Debug("docker image inspect file list", "error", err)
				continue
			}
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

	path = make([]string, 0)
	pathInfo = function.PluckArrayWalk(pathInfo, func(i *FileItemResult) (*FileItemResult, bool) {
		if function.InArray(path, i.Name) {
			return nil, false
		} else {
			path = append(path, i.Name)
			return i, true
		}
	})
	return pathInfo, path, nil
}

func getFileListFromTar(tarReader *tar.Reader) (files []*FileItemResult, err error) {
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
			files = append(files, &FileItemResult{
				ShowName: filepath.Base(header.Name),
				Name:     filepath.Join("/", header.Name),
				LinkName: header.Linkname,
				Size:     units.BytesSize(float64(header.Size)),
				Mode:     fmt.Sprintf("%d", header.Mode),
				IsDir:    header.Typeflag == tar.TypeDir,
				ModTime:  header.ModTime.String(),
				Change:   0,
				Group:    fmt.Sprintf("%d", header.Gid),
				Owner:    fmt.Sprintf("%d", header.Uid),
			})
		}
	}
	return files, nil
}
