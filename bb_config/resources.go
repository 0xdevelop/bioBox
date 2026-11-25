package bb_config

import (
	"github.com/0xdevelop/bioBox/bb_common/bb_app_path"
	"github.com/0xdevelop/bioBox/bb_common/bb_resource"
	"github.com/0xdevelop/bioBox/bb_runtime_resource"
	"path/filepath"
	"strings"
)

func ConfiguredResourceSets() map[string]*bb_resource.Config {
	sets := map[string]*bb_resource.Config{}
	if GlobalConfig == nil || GlobalConfig.CliCfg == nil {
		return sets
	}
	c := GlobalConfig.CliCfg
	if c.Protenix != nil {
		sets["protenix.v1"] = c.Protenix.Resources
		for version, runtime := range c.Protenix.VersionRuntimes {
			if runtime == nil {
				continue
			}
			r := runtime.Resources
			if r == nil {
				r = c.Protenix.Resources
			}
			sets["protenix."+version] = r
		}
	}
	if c.PXDesign != nil {
		sets["pxdesign.preview"] = c.PXDesign.Resources
	}
	root, err := bb_app_path.ExecutableDir()
	if err != nil {
		return nil
	}
	for id, r := range sets {
		var binary string
		var extra map[string]string
		kind := "protenix"
		if id == "pxdesign.preview" {
			kind = "pxdesign"
			binary = c.PXDesign.Binary
			extra = c.PXDesign.ExtraEnv
		} else {
			binary = c.Protenix.Binary
			extra = c.Protenix.ExtraEnv
			version := strings.TrimPrefix(id, "protenix.")
			if rt := c.Protenix.VersionRuntimes[version]; rt != nil {
				binary = rt.Binary
				if rt.ExtraEnv != nil {
					extra = rt.ExtraEnv
				}
			}
			if version != "v1" && filepath.Base(binary) == "protenix-run" {
				sets[id] = nil
				continue
			}
		}
		compiled, err := bb_runtime_resource.Compile(kind, binary, root, extra, r)
		if err != nil {
			sets[id] = nil
		} else {
			sets[id] = compiled
		}
	}
	return sets
}

// VerifyResourceRuntime is used only by the official wrapper's read-only verify.
func VerifyResourceRuntime(id string) error {
	if GlobalConfig == nil || GlobalConfig.CliCfg == nil {
		return bb_resource.ErrSetUnavailable
	}
	c := GlobalConfig.CliCfg
	kind := "protenix"
	var binary string
	switch id {
	case "protenix.v1":
		if c.Protenix == nil {
			return bb_resource.ErrSetUnavailable
		}
		binary = c.Protenix.Binary
		if rt := c.Protenix.VersionRuntimes["v1"]; rt != nil {
			binary = rt.Binary
		}
	case "pxdesign.preview":
		kind = "pxdesign"
		if c.PXDesign == nil {
			return bb_resource.ErrSetUnavailable
		}
		binary = c.PXDesign.Binary
	default:
		return bb_runtime_resource.ErrUnsupported
	}
	root, err := bb_app_path.ExecutableDir()
	if err != nil {
		return err
	}
	return bb_runtime_resource.ValidateInstalled(kind, binary, root)
}

func VerifyResourceRuntimeBinding(id, path string) error {
	if GlobalConfig == nil || GlobalConfig.CliCfg == nil {
		return bb_resource.ErrSetUnavailable
	}
	c := GlobalConfig.CliCfg
	var binary string
	switch id {
	case "protenix.v1":
		if c.Protenix == nil {
			return bb_resource.ErrSetUnavailable
		}
		binary = c.Protenix.Binary
		if rt := c.Protenix.VersionRuntimes["v1"]; rt != nil {
			binary = rt.Binary
		}
	case "pxdesign.preview":
		if c.PXDesign == nil {
			return bb_resource.ErrSetUnavailable
		}
		binary = c.PXDesign.Binary
	default:
		return bb_runtime_resource.ErrUnsupported
	}
	root, err := bb_app_path.ExecutableDir()
	if err != nil {
		return err
	}
	expected, err := bb_resource.CanonicalPath(root, binary)
	if err != nil {
		return err
	}
	actual, err := bb_resource.CanonicalPath(root, path)
	if err != nil {
		return err
	}
	if actual != expected {
		return bb_runtime_resource.ErrUnsupported
	}
	return nil
}
