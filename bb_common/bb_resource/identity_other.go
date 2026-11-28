//go:build !unix

package bb_resource

import "os"

func applyPlatformIdentity(id *fileIdentityStamp, st os.FileInfo) {}
