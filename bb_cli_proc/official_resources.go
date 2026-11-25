package bb_cli_proc

import (
	"github.com/0xdevelop/bioBox/bb_common/bb_app_path"
	"github.com/0xdevelop/bioBox/bb_runtime_resource"
	"path/filepath"
)

func validateOfficialResources(s Spec) error {
	if (s.Name != "protenix" && s.Name != "pxdesign") || filepath.Base(s.Path) != s.Name+"-run" {
		return nil
	}
	root := s.ResourceRoot
	var err error
	if root == "" {
		root, err = bb_app_path.ExecutableDir()
		if err != nil {
			return err
		}
	}
	if err = bb_runtime_resource.ValidateManifest(s.Name, s.Path, root, s.Env, s.Resources); err != nil {
		return err
	}
	if err = bb_runtime_resource.ValidateArguments(s.Name, s.Args); err != nil {
		return err
	}
	return bb_runtime_resource.ValidateInstalled(s.Name, s.Path, root)
}
