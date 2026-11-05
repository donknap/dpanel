package imagefs

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/docker/docker/client"
	filedata "github.com/donknap/dpanel/common/types/fs"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

type tarEntry struct {
	info     filedata.FileData
	hardLink string
}

func (self *Fs) loadTar(dockerClient *client.Client, temporaryDir string) (err error) {
	if err = os.MkdirAll(temporaryDir, 0o700); err != nil {
		return err
	}
	self.directoryPath, err = os.MkdirTemp(temporaryDir, "imagefs-")
	if err != nil {
		return err
	}
	archivePath := filepath.Join(self.directoryPath, "image.tar")
	archive, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	self.archivePath = archivePath
	stream, err := dockerClient.ImageSave(self.ctx, []string{self.name})
	if err != nil {
		return errors.Join(err, archive.Close())
	}
	_, err = io.Copy(archive, stream)
	err = errors.Join(err, stream.Close(), archive.Close())
	if err != nil {
		return err
	}
	image, err := tarball.ImageFromPath(archivePath, nil)
	if err != nil {
		return err
	}
	layerList, err := image.Layers()
	if err != nil {
		return err
	}
	self.entryMap = map[string]*tarEntry{
		"/": {info: filedata.FileData{Path: "/", Name: "/", Mod: os.ModeDir | 0o755, ModStr: "drwxr-xr-x", IsDir: true, Change: filedata.ChangeDefault}},
	}
	for index, layer := range layerList {
		if err = self.ctx.Err(); err != nil {
			return err
		}
		stream, err := layer.Uncompressed()
		if err != nil {
			return err
		}
		reader := &imageReader{ctx: self.ctx, reader: stream}
		err = self.applyLayer(reader)
		if err == nil {
			_, err = io.Copy(io.Discard, reader)
		}
		err = errors.Join(err, stream.Close())
		if err != nil {
			return fmt.Errorf("read image layer %d: %w", index, err)
		}
	}
	return self.ctx.Err()
}

func (self *Fs) applyLayer(stream io.Reader) error {
	reader := tar.NewReader(stream)
	entryList := make([]*tarEntry, 0)
	for {
		if err := self.ctx.Err(); err != nil {
			return err
		}
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		name, err := imageTarPath(header.Name)
		if err != nil {
			return err
		}
		baseName := path.Base(name)
		if strings.HasPrefix(baseName, ".wh.") {
			// Whiteout 只移除旧层内容，同层新文件在扫描结束后统一应用。
			if baseName == ".wh..wh..opq" {
				self.removeEntry(path.Dir(name), false)
			} else {
				target := strings.TrimPrefix(baseName, ".wh.")
				if target == "" || target == "." || target == ".." {
					return errors.New("invalid image whiteout")
				}
				self.removeEntry(path.Join(path.Dir(name), target), true)
			}
			continue
		}
		if name == "/" && header.Typeflag != tar.TypeDir {
			return errors.New("image root must be a directory")
		}
		mode := header.FileInfo().Mode()
		entry := &tarEntry{
			info: filedata.FileData{
				Path: name, Name: path.Base(name), Mod: mode, ModStr: mode.String(),
				ModTime: header.ModTime, Size: header.Size, UID: uint32(header.Uid), GID: uint32(header.Gid),
				User: header.Uname, Group: header.Gname, IsDir: mode.IsDir(),
				IsSymlink: header.Typeflag == tar.TypeSymlink, LinkName: header.Linkname,
				Change: filedata.ChangeDefault,
			},
		}
		if entry.info.User == "" {
			entry.info.User = fmt.Sprint(header.Uid)
		}
		if entry.info.Group == "" {
			entry.info.Group = fmt.Sprint(header.Gid)
		}
		if entry.info.IsSymlink {
			entry.info.Size = int64(len(header.Linkname))
		}
		if header.Typeflag == tar.TypeLink {
			entry.hardLink, err = imageTarPath(header.Linkname)
			if err != nil {
				return err
			}
		}
		entryList = append(entryList, entry)
	}
	for _, entry := range entryList {
		for parent := path.Dir(entry.info.Path); parent != "/"; parent = path.Dir(parent) {
			if existing, ok := self.entryMap[parent]; ok {
				if !existing.info.IsDir {
					return fmt.Errorf("image entry parent %s is not a directory", parent)
				}
				break
			}
			self.entryMap[parent] = &tarEntry{info: filedata.FileData{
				Path: parent, Name: path.Base(parent), Mod: os.ModeDir | 0o755, ModStr: "drwxr-xr-x",
				IsDir: true, Change: filedata.ChangeDefault,
			}}
		}
		if existing, ok := self.entryMap[entry.info.Path]; ok && existing.info.IsDir && !entry.info.IsDir {
			self.removeEntry(entry.info.Path, false)
		}
		if entry.hardLink != "" {
			if target, ok := self.entryMap[entry.hardLink]; ok && target.hardLink == "" {
				if !target.info.Mod.IsRegular() {
					return fmt.Errorf("image hard link target %s is not a regular file", entry.hardLink)
				}
				entry.copyLinkInfo(target)
			}
		}
		self.entryMap[entry.info.Path] = entry
	}
	// 允许同层硬链接先于目标出现；循环或缺失目标不能成为成功的文件索引。
	for {
		remaining := 0
		progress := 0
		for _, entry := range self.entryMap {
			if entry.hardLink == "" {
				continue
			}
			remaining++
			if target, ok := self.entryMap[entry.hardLink]; ok && target.hardLink == "" && target.info.Mod.IsRegular() {
				entry.copyLinkInfo(target)
				progress++
			}
		}
		if remaining == 0 {
			break
		}
		if progress == 0 {
			return errors.New("image contains unresolved hard links")
		}
	}
	return nil
}

func imageTarPath(name string) (string, error) {
	if name == "" || strings.ContainsRune(name, 0) || path.IsAbs(name) {
		return "", fmt.Errorf("invalid image archive path %q", name)
	}
	name = path.Clean(name)
	if name == ".." || strings.HasPrefix(name, "../") {
		return "", fmt.Errorf("invalid image archive path %q", name)
	}
	return path.Join("/", name), nil
}

func (self *Fs) removeEntry(name string, includeSelf bool) {
	for entryPath := range self.entryMap {
		if includeSelf && entryPath == name || entryPath != name && strings.HasPrefix(entryPath, strings.TrimSuffix(name, "/")+"/") {
			delete(self.entryMap, entryPath)
		}
	}
}

func (self *tarEntry) copyLinkInfo(target *tarEntry) {
	name := self.info.Path
	self.info = target.info
	self.info.Path, self.info.Name = name, path.Base(name)
	self.hardLink = ""
}

type imageReader struct {
	ctx    context.Context
	reader io.Reader
}

func (self *imageReader) Read(buffer []byte) (int, error) {
	if err := self.ctx.Err(); err != nil {
		return 0, err
	}
	return self.reader.Read(buffer)
}
