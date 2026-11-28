//go:build linux

package bb_resource

import (
	"os"
	"syscall"
)

func applyPlatformIdentity(id *fileIdentityStamp, st os.FileInfo) {
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return
	}
	id.Inode = uint64(sys.Ino)
	id.CtimeNsec = sys.Ctim.Sec*1e9 + int64(sys.Ctim.Nsec)
}
