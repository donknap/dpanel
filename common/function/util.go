package function

import (
	"crypto/md5"
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
)

func GetRandomString(n int) string {
	str := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz123456789"
	bytes := []byte(str)
	var result []byte
	for i := 0; i < n; i++ {
		result = append(result, bytes[rand.Intn(len(bytes))])
	}
	return string(result)
}

func GetMd5(str string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(str)))
}

type pathInfoOut struct {
	DirName   string
	BaseName  string
	Extension string
	Filename  string
}

func GetPathInfo(path string) *pathInfoOut {
	filename := filepath.Base(path)
	ext := filepath.Ext(filename)

	dirname, basename := filepath.Split(path)
	basename = basename[:len(basename)-len(ext)]
	result := &pathInfoOut{}
	result.DirName = dirname
	result.BaseName = basename
	result.Extension = ext
	result.Filename = filename
	return result
}

func CheckFileAllowUpload(filename string) bool {
	allowFileExt := []string{
		".zip", ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".cvs",
		".jpg", ".png", ".jpeg", ".gif",
	}
	for _, s := range allowFileExt {
		if strings.HasSuffix(filename, s) {
			return true
		}
	}
	return false
}

func GetRootPath() string {
	rootPath, _ := filepath.Abs("./")
	return rootPath
}
