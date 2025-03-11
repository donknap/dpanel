package docker

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"github.com/docker/go-units"
	"github.com/donknap/dpanel/common/function"
	"github.com/mcuadros/go-version"
	"io"
	"path"
	"strings"
)

func (self Builder) ImageInspectFileList(imageID string) (fileList []*FileItemResult, err error) {
	imageInfo, err := self.Client.ImageInspect(self.Ctx, imageID)
	if err != nil {
		return nil, err
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
		if version.Compare(dockerVersion.Version, "25", ">=") && function.InArray(layers, header.Name) {
			fileList, err = getFileListFromTar(tar.NewReader(tarReader))
			if err != nil {
				return nil, err
			}
			return fileList, nil
		} else if strings.HasSuffix(header.Name, ".tar") {
			fileList, err = getFileListFromTar(tar.NewReader(tarReader))
			if err != nil {
				return nil, err
			}
			return fileList, nil
		} else if strings.HasSuffix(header.Name, ".tar.gz") || strings.HasSuffix(header.Name, "tgz") {
			gzReader, err := gzip.NewReader(tarReader)
			if err != nil {
				return nil, err
			}
			fileList, err = getFileListFromTar(tar.NewReader(gzReader))
			_ = gzReader.Close()
			if err != nil {
				return nil, err
			}
			return fileList, nil
		}
	}
	return fileList, nil
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
				ShowName: header.Name,
				Name:     header.Name,
				LinkName: header.Linkname,
				Size:     units.BytesSize(float64(header.Size)),
				Mode:     fmt.Sprintf("%d", header.Mode),
				IsDir:    header.Typeflag == tar.TypeDir,
				ModTime:  header.ModTime.Location().String(),
				Change:   0,
				Group:    fmt.Sprintf("%d", header.Gid),
				Owner:    fmt.Sprintf("%d", header.Uid),
			})
		}
	}
	return files, nil
}
