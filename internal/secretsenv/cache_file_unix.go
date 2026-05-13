//go:build !windows

package secretsenv

import (
	"fmt"
	"os"
	"syscall"
)

func replaceFile(tmpName, path string) error {
	return os.Rename(tmpName, path)
}

func verifyCacheDirOwner(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("inspect cache dir owner: unsupported stat type")
	}
	uid := uint32(os.Getuid())
	if stat.Uid != uid {
		return fmt.Errorf("cache dir owner uid %d does not match current uid %d", stat.Uid, uid)
	}
	return nil
}

func cacheDirModeIsPrivate(info os.FileInfo) bool {
	return info.Mode().Perm()&0o077 == 0
}
