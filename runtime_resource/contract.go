// Package runtime_resource owns pinned Python resource knowledge, not the task engine.
package runtime_resource

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/0xdevelop/bioBox/common/resource"
	"os"
	"path/filepath"
	"strings"
)

var ErrUnsupported = errors.New("runtime resource branch unsupported; Python was not started")

func RuntimeRoot(binary, root string) (string, error) {
	p, err := resource.CanonicalPath(root, binary)
	if err != nil {
		return "", err
	}
	return filepath.Dir(filepath.Dir(p)), nil
}
func values(kind, binary, root string) (map[string]string, error) {
	prefix, err := RuntimeRoot(binary, root)
	if err != nil {
		return nil, err
	}
	vals := map[string]string{}
	if kind == "protenix" {
		vals["PROTENIX_MODEL"] = "protenix_base_default_v1.0.0"
		vals["PROTENIX_ROOT_DIR"] = filepath.Join(prefix, "data")
	} else {
		vals["PXDESIGN_DATA"] = filepath.Join(prefix, "data")
	}
	in, err := os.Open(filepath.Join(prefix, "conf", kind+".env"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if in != nil {
		defer in.Close()
		scanner := bufio.NewScanner(in)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if strings.HasPrefix(line, "export ") || strings.HasPrefix(line, "export\t") {
				line = strings.TrimSpace(line[len("export"):])
			}
			key, value, ok := strings.Cut(line, "=")
			key = strings.TrimSpace(key)
			if !ok || key == "" || strings.ContainsAny(key, " \t;|&()'\"") {
				return nil, errors.New("runtime env must contain literal assignments only")
			}
			value, err = parseLiteral(value)
			if err != nil {
				return nil, err
			}
			switch key {
			case "PROTENIX_MODEL", "PROTENIX_ROOT_DIR", "PXDESIGN_DATA", "PXDESIGN_CHECKPOINT_DIR", "TOOL_WEIGHTS_ROOT", "PROTENIX_DATA_ROOT_DIR":
			default:
				continue
			}
			if strings.Contains(value, "@PREFIX@") {
				return nil, errors.New("unresolved runtime prefix placeholder")
			}

			vals[key] = value
		}
		if err = scanner.Err(); err != nil {
			return nil, err
		}
	}
	if kind == "protenix" {
		vals["PROTENIX_ROOT_DIR"], err = resource.Resolve(prefix, vals["PROTENIX_ROOT_DIR"])
		return vals, err
	}
	data, err := resource.Resolve(prefix, vals["PXDESIGN_DATA"])
	if err != nil {
		return nil, err
	}
	vals["PXDESIGN_DATA"] = data
	for k, def := range map[string]string{"PXDESIGN_CHECKPOINT_DIR": filepath.Join(data, "release_data/checkpoint"), "TOOL_WEIGHTS_ROOT": filepath.Join(data, "tool_weights"), "PROTENIX_DATA_ROOT_DIR": filepath.Join(data, "protenix_data")} {
		v := vals[k]
		if v == "" {
			v = def
		}
		vals[k], err = resource.Resolve(prefix, v)
		if err != nil {
			return nil, err
		}
	}
	return vals, nil
}

// Required is a finite pinned profile. Conditional MSA/template/ESM/PTX modes
// remain unsupported; no guessed hashes or downloads are added here.
func Required(kind, binary, root string) ([]string, error) {
	vals, err := values(kind, binary, root)
	if err != nil {
		return nil, err
	}
	files := []string{}
	if kind == "protenix" {
		if vals["PROTENIX_MODEL"] != "protenix_base_default_v1.0.0" {
			return nil, ErrUnsupported
		}
		data := vals["PROTENIX_ROOT_DIR"]
		for _, f := range []string{"common/components.cif", "common/components.cif.rdkit_mol.pkl", "common/clusters-by-entity-40.txt", "common/obsolete_release_date.csv", "checkpoint/protenix_base_default_v1.0.0.pt"} {
			files = append(files, filepath.Join(data, f))
		}
		return files, nil
	}
	if kind != "pxdesign" {
		return nil, ErrUnsupported
	}
	for _, m := range []string{"pxdesign_v0.1.0", "protenix_base_default_v0.5.0", "protenix_mini_default_v0.5.0", "protenix_mini_tmpl_v0.5.0"} {
		files = append(files, filepath.Join(vals["PXDESIGN_CHECKPOINT_DIR"], m+".pt"))
	}
	data := vals["PROTENIX_DATA_ROOT_DIR"]
	// Upstream prefers unversioned CCD names if present, otherwise versioned names.
	for _, f := range []string{"components.cif", "components.cif.rdkit_mol.pkl"} {
		selected := filepath.Join(data, f)
		if _, e := os.Stat(selected); os.IsNotExist(e) {
			selected = filepath.Join(data, strings.Replace(f, "components.cif", "components.v20240608.cif", 1))
		} else if e != nil {
			return nil, e
		}
		files = append(files, selected)
	}
	files = append(files, filepath.Join(data, "clusters-by-entity-40.txt"))
	for _, f := range []string{"af2/params_model_1.npz", "af2/params_model_1_ptm.npz", "mpnn/ca_model_weights/v_48_020.pt", "mpnn/vanilla_model_weights/v_48_020.pt"} {
		files = append(files, filepath.Join(vals["TOOL_WEIGHTS_ROOT"], f))
	}
	return files, nil
}
func ValidateManifest(kind, binary, root string, extra map[string]string, cfg *resource.Config) error {
	if cfg == nil || !cfg.Enabled || len(cfg.Files) == 0 {
		return resource.ErrSetUnavailable
	}
	// An explicitly configured external executable is covered only by its declared
	// generic manifest, not by the official Python profile's completeness proof.
	if filepath.Base(binary) != kind+"-run" {
		return nil
	}
	for _, k := range []string{"PROTENIX_MODEL", "PROTENIX_ROOT_DIR", "PXDESIGN_DATA", "PXDESIGN_CHECKPOINT_DIR", "TOOL_WEIGHTS_ROOT", "PROTENIX_DATA_ROOT_DIR"} {
		if _, ok := extra[k]; ok {
			return fmt.Errorf("owned resource path %s belongs in runtime conf, not inherited env", k)
		}
	}
	want, err := Required(kind, binary, root)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, f := range cfg.Files {
		path, e := resource.CanonicalPath(root, f.Path)
		if e != nil {
			return e
		}
		have[path] = true
	}
	for _, f := range want {
		path, e := resource.CanonicalPath(root, f)
		if e != nil {
			return e
		}
		if !have[path] {
			return fmt.Errorf("official %s manifest incomplete: required %s", kind, filepath.Base(f))
		}
	}
	return nil
}
func ValidateArguments(kind string, args []string) error {
	get := func(key string) string {
		for i, a := range args {
			if a == key && i+1 < len(args) {
				return strings.ToLower(args[i+1])
			}
			if strings.HasPrefix(a, key+"=") {
				return strings.ToLower(strings.TrimPrefix(a, key+"="))
			}
		}
		return ""
	}
	if get("--use_msa") != "false" {
		return fmt.Errorf("%w: explicit --use_msa false required until MSA coordination exists", ErrUnsupported)
	}
	for _, key := range []string{"--use_template", "--use_rna_msa"} {
		if v := get(key); v != "" && v != "false" {
			return ErrUnsupported
		}
	}
	if kind == "protenix" {
		if model := get("--model_name"); model != "" && model != "protenix_base_default_v1.0.0" {
			return ErrUnsupported
		}
	} else {
		if preset := get("--preset"); preset != "" && preset != "preview" {
			return ErrUnsupported
		}
	}
	return nil
}

func parseLiteral(value string) (string, error) {
	value = strings.TrimSpace(value)
	quoted := false
	var delim byte
	if len(value) > 0 && (value[0] == '\'' || value[0] == '"') {
		delim = value[0]
		if len(value) < 2 || value[len(value)-1] != delim {
			return "", errors.New("runtime env literal quote mismatch")
		}
		quoted = true
		value = value[1 : len(value)-1]
	}
	if strings.ContainsAny(value, "$`\\\r\n") || (!quoted && strings.ContainsAny(value, " \t#;|&<>()'\"")) || (quoted && strings.ContainsRune(value, rune(delim))) {
		return "", errors.New("runtime env expansion or shell expression unsupported")
	}
	return value, nil
}

// Compile binds stable instance resource IDs to runtime-conf roots. Changing one
// .env root value redirects the whole family without repeating every file path.
func Compile(kind, binary, root string, extra map[string]string, cfg *resource.Config) (*resource.Config, error) {
	if cfg == nil || !cfg.Enabled || len(cfg.Files) == 0 {
		return nil, resource.ErrSetUnavailable
	}
	cloned := *cfg
	cloned.Files = append([]resource.File(nil), cfg.Files...)
	if filepath.Base(binary) == kind+"-run" {
		required, err := Required(kind, binary, root)
		if err != nil {
			return nil, err
		}
		roles := map[string]string{}
		for _, p := range required {
			base := filepath.Base(p)
			id := base
			switch base {
			case "components.cif", "components.v20240608.cif":
				id = "ccd_components_file"
			case "components.cif.rdkit_mol.pkl", "components.v20240608.cif.rdkit_mol.pkl":
				id = "ccd_components_rdkit_mol_file"
			case "clusters-by-entity-40.txt":
				id = "pdb_cluster_file"
			case "obsolete_release_date.csv":
				id = "obsolete_release_data_csv"
			case "protenix_base_default_v1.0.0.pt", "pxdesign_v0.1.0.pt":
				id = "model"
			case "params_model_1.npz":
				id = "af2_model_1"
			case "params_model_1_ptm.npz":
				id = "af2_model_1_ptm"
			case "v_48_020.pt":
				if strings.Contains(p, "/ca_model_weights/") {
					id = "mpnn_ca"
				} else {
					id = "mpnn_bb"
				}
			}
			roles[id] = p
		}
		for i, f := range cloned.Files {
			if f.ID == "" {
				continue
			}
			path := roles[f.ID]
			if path == "" {
				return nil, fmt.Errorf("unknown resource id %s", f.ID)
			}
			if f.Path != "" {
				actual, e := resource.CanonicalPath(root, f.Path)
				if e != nil {
					return nil, e
				}
				expected, e := resource.CanonicalPath(root, path)
				if e != nil {
					return nil, e
				}
				if actual != expected {
					return nil, errors.New("resource id/path conflict; put family roots in runtime conf")
				}
			}
			cloned.Files[i].Path = path
		}
	}
	if err := ValidateManifest(kind, binary, root, extra, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}
