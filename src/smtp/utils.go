package smtp

import (
	"os"
	"io"
)

func CopyFile(src, dst string) (written int64, err error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return
	}
	defer srcFile.Close()
	dstFile, err := os.Create(dst)
	if err != nil {
		return
	}
	defer dstFile.Close()
	return io.Copy(srcFile, dstFile)
}

func MoveFile(src, dst string) (err error) {
	err = os.Rename(src, dst)
	if err == nil {
		return
	}
	_, err = CopyFile(src, dst)
	if err != nil {
		return
	}
	return os.Remove(src)
}
