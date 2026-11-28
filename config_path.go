package main

import "github.com/george012/gtbox"

// configFilePathForRunMode keeps debug on the CWD-relative example file used by
// GoLand, and leaves test/release on the executable-dir path already selected
// by setupApp.
func configFilePathForRunMode(runMode gtbox.RunMode, setupPath string) string {
	if runMode == gtbox.RunModeDebug {
		return "./example_files/config_local.yaml"
	}
	return setupPath
}
