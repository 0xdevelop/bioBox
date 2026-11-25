package bb_runtime_resource

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// These are source-code equality guards for the inspected release/installed
// revisions, NOT official model hashes or a new upstream-authenticity claim.
var inspectedCode = []struct{ kind, location, path, digest string }{
	{"protenix", "site", "runner/inference.py", "9cf94eb80124056e827ad839fdc78c432955d9108c3ad08dfe813a420fde25e3"},
	{"protenix", "site", "runner/batch_inference.py", "c28ae66131f97d7bcdd16c71ab55f4f2afd3f05fecccd0b152754459efba268c"},
	{"protenix", "site", "protenix/web_service/colab_request_parser.py", "cc22484679a3a95e20af30f2154f64a974faae7ef375de2f7801f076596acb6d"},
	{"pxdesign", "source", "pxdesign/utils/infer.py", "de07913dfdc266bdca6b9641ec1617536567064f401a031e078200caac9a44ff"},
	{"pxdesign", "source", "pxdesign/runner/pipeline.py", "ab7916051254fdc1915440c187b3a8d74c900387df01a4e8f8e08970a73e9a82"},
	{"pxdesign", "source", "pxdesign/runner/cli.py", "274f816bb6b21346a9085c15af8b6b2d64fffccee5d39c64b79e0b2c9207d74c"},
	{"pxdesign", "source", "pxdesign/configs/configs_infer.py", "5b082f9e3ceacdaf31c8b28d81234213a264148b75fc0c50dffb719489a50bb2"},
	{"pxdesign", "source", "pxdesign/configs/configs_data.py", "335bb3f0e15776d94259c6cc1c8968db7b2626a573450caae476d7f09d3d9e3b"},
	{"pxdesign", "site", "pxdbench/tools/ptx/ptx_utils.py", "8f9b53b3b85a3f0394a90fa575048c4f1a638be6d63414cf7909c9904b2c57d0"},
	{"pxdesign", "site", "pxdbench/pxd_configs/eval.py", "9a07e3c4c1b5cc39d061ccaf23b344abc664df9b1e1ae42291df8262bf843184"},
	{"pxdesign", "site", "pxdbench/globals.py", "845c381e398059afcc4faf29b483889c0afcce079d4a59d4357d8add0c993fb8"},
	{"pxdesign", "site", "colabdesign/mpnn/model.py", "301fb7186c3f5c5a2914a0cd57ef4aea64d5d47b9d2e8be6a9a82e8c5842a8dd"},
	{"pxdesign", "site", "colabdesign/af/model.py", "d0e7aa65e491f6744e4e8ef26f01a88bc731855f773e68bc3a0ca46cc29e1758"},
	{"pxdesign", "site", "colabdesign/af/alphafold/model/data.py", "e532dab64bff98280611f1dc152ee72c556a992abebc778db1c881560cc70936"},
}

func ValidateInstalled(kind, binary, root string) error {
	if filepath.Base(binary) != kind+"-run" {
		return nil
	}
	prefix, err := RuntimeRoot(binary, root)
	if err != nil {
		return err
	}
	release := filepath.Join(prefix, "current")
	if _, err = os.Stat(filepath.Join(release, "venv/bin/python")); os.IsNotExist(err) {
		release = prefix
	} else if err != nil {
		return err
	}
	site := filepath.Join(release, "venv/lib/python3.14/site-packages")
	source := filepath.Join(release, "src/pxdesign-f788441313c8")
	for _, f := range inspectedCode {
		if f.kind != kind {
			continue
		}
		base := site
		if f.location == "source" {
			base = source
		}
		data, err := os.ReadFile(filepath.Join(base, f.path))
		if err != nil {
			return fmt.Errorf("%w: inspected runtime code missing: %s", ErrUnsupported, f.path)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != f.digest {
			return fmt.Errorf("%w: runtime revision differs: %s", ErrUnsupported, f.path)
		}
	}
	return nil
}
