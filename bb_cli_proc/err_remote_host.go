package bb_cli_proc

import (
	"fmt"
	"strings"
)

// ErrRemoteHostUnsupported is returned when Spec.Host is set. Staging and
// artifact reads currently happen only on the bioBox host.
func errRemoteHostUnsupported(name, host string) error {
	label := strings.TrimSpace(name)
	if label == "" {
		label = "cli"
	}
	return fmt.Errorf("%s: remote host %q is set, but this build only stages inputs and reads artifacts on the local machine; cross-host file transfer and remote process management are not implemented", label, host)
}
