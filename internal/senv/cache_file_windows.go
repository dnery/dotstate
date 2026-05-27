//go:build windows

package senv

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	moveFileReplaceExisting = 0x1
	moveFileWriteThrough    = 0x8
)

var moveFileExW = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

func replaceFile(tmpName, path string) error {
	from, err := syscall.UTF16PtrFromString(tmpName)
	if err != nil {
		return err
	}
	to, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	r1, _, callErr := moveFileExW.Call(
		uintptr(unsafe.Pointer(from)),
		uintptr(unsafe.Pointer(to)),
		uintptr(moveFileReplaceExisting|moveFileWriteThrough),
	)
	if r1 == 0 {
		if callErr != syscall.Errno(0) {
			return callErr
		}
		return syscall.EINVAL
	}
	return nil
}

func verifyCacheDirOwner(os.FileInfo) error {
	return fmt.Errorf("windows cache ACL privacy verification is not implemented; refusing to write secret cache")
}

func cacheDirModeIsPrivate(os.FileInfo) bool {
	return false
}
