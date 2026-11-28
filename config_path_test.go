package main

import (
	"testing"

	"github.com/george012/gtbox"
)

func TestConfigFilePathForRunMode(t *testing.T) {
	setup := "/opt/biobox/conf/config.yaml"
	if got := configFilePathForRunMode(gtbox.RunModeDebug, setup); got != "./example_files/config_local.yaml" {
		t.Fatalf("debug path: %s", got)
	}
	for _, mode := range []gtbox.RunMode{gtbox.RunModeTest, gtbox.RunModeRelease} {
		if got := configFilePathForRunMode(mode, setup); got != setup {
			t.Fatalf("%s should keep setupApp path, got %s", mode, got)
		}
		if got := configFilePathForRunMode(mode, setup); got == "./conf/config.yaml" {
			t.Fatalf("%s must not fall back to CWD-relative ./conf/config.yaml", mode)
		}
	}
}
