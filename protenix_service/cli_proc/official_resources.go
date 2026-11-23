package cli_proc

import (
	"github.com/0xdevelop/protenix_service/common/app_path"
	"github.com/0xdevelop/protenix_service/runtime_resource"
	"path/filepath"
)

func validateOfficialResources(s Spec) error {
	if (s.Name != "protenix" && s.Name != "pxdesign") || filepath.Base(s.Path) != s.Name+"-run" {
		return nil
	}
	root := s.ResourceRoot
	var err error
	if root == "" {
		root, err = app_path.ExecutableDir()
		if err != nil {
			return err
		}
	}
	if err = runtime_resource.ValidateManifest(s.Name, s.Path, root, s.Env, s.Resources); err != nil {
		return err
	}
	if err = runtime_resource.ValidateArguments(s.Name, s.Args); err != nil {
		return err
	}
	return runtime_resource.ValidateInstalled(s.Name, s.Path, root)
}
