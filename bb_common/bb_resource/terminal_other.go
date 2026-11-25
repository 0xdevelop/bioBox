//go:build !linux

package bb_resource

import "os"

// Other platforms use explicit scoped --confirm instead of guessing TTY status.
func IsTerminal(file *os.File) bool { return false }
