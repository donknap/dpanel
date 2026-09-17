package fs

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

func TestCopyRejectsDirectoryAlias(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires additional privileges on Windows")
	}
	rootPath := t.TempDir()
	if err := os.Mkdir(filepath.Join(rootPath, "source"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("source", filepath.Join(rootPath, "alias")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err = NewCpCommand().copyPath(root, "source", "alias/child", false, true); err == nil {
		t.Fatal("copy into a symlink alias of the source succeeded")
	}
	if _, err = os.Lstat(filepath.Join(rootPath, "source", "child")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("recursive destination was created: %v", err)
	}
}

func TestCopyPreflightKeepsTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("named pipes are not supported on Windows")
	}
	rootPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootPath, "target"), []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(rootPath, "unsupported"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err = NewCpCommand().copyPath(root, "unsupported", "target", false, true); err == nil {
		t.Fatal("unsupported source was copied")
	}
	content, err := os.ReadFile(filepath.Join(rootPath, "target"))
	if err != nil || string(content) != "original" {
		t.Fatalf("target changed: content=%q err=%v", content, err)
	}
}

func TestRemoveRefusesRoot(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootPath, "sentinel"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err = NewRmCommand().Run(root, options{path: "/", recursive: true}); err == nil {
		t.Fatal("root removal succeeded")
	}
	if content, err := os.ReadFile(filepath.Join(rootPath, "sentinel")); err != nil || string(content) != "keep" {
		t.Fatalf("sentinel changed: content=%q err=%v", content, err)
	}
}
