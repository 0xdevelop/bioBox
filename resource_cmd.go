package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/0xdevelop/bioBox/bb_common/bb_app_path"
	"github.com/0xdevelop/bioBox/bb_common/bb_resource"
	"github.com/0xdevelop/bioBox/bb_config"
	"os"
	"os/signal"
	"syscall"
)

// Product resource command runs before normal app setup: no server/storage/systemd startup.
func resourceCommand(args []string) int {
	root, err := bb_app_path.ExecutableDir()
	if err != nil {
		return 1
	}
	fs := flag.NewFlagSet("resource", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	runtimePath := fs.String("runtime", "", "expected official wrapper path for verification")
	file := fs.String("config", "conf/config.yaml", "configuration file, relative to executable directory")
	if fs.Parse(args) != nil {
		return 2
	}
	path, err := bb_app_path.Resolve(root, *file)
	if err != nil {
		return 1
	}
	if err = bb_config.LoadConfig(path); err != nil {
		fmt.Fprintln(os.Stderr, "cannot load resource configuration (contents suppressed)")
		return 1
	}
	if rest := fs.Args(); len(rest) > 1 && rest[0] == "verify" {
		if *runtimePath != "" {
			if err = bb_config.VerifyResourceRuntimeBinding(rest[1], *runtimePath); err != nil {
				fmt.Fprintln(os.Stderr, "configured resource/runtime binding mismatch")
				return 1
			}
		}
		if err = bb_config.VerifyResourceRuntime(rest[1]); err != nil {
			fmt.Fprintln(os.Stderr, "runtime revision not verified; Python was not started")
			return 1
		}
	}
	interactive := bb_resource.IsTerminal(os.Stdin)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return bb_resource.RunCLI(ctx, bb_resource.Controller{Root: root, Sets: bb_config.ConfiguredResourceSets()}, fs.Args(), os.Stdin, os.Stdout, os.Stderr, interactive)
}
