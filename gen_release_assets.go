//go:build ignore

// Release packaging gate, invoked explicitly with go run; never installs or publishes.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"debug/buildinfo"
	"debug/elf"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type asset struct {
	Name   string `json:"name"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}
type manifest struct {
	Schema       int     `json:"schema"`
	Version      string  `json:"version"`
	Commit       string  `json:"commit"`
	Architecture string  `json:"architecture"`
	NativeHost   string  `json:"native_host,omitempty"`
	Assets       []asset `json:"assets"`
}

var arches = []string{"amd64", "arm64"}
var products = []string{"protenix", "pxdesign", "protenix_service"}
var versionRE = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var shaRE = regexp.MustCompile(`^[a-f0-9]{40}$`)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "[release-assets]", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: go run gen_release_assets.go version|arch|aggregate|verify|uploaded|self-test [flags]")
	}
	if args[0] == "self-test" {
		if len(args) != 1 {
			return errors.New("self-test takes no arguments")
		}
		return selfTest()
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	dir := fs.String("dir", "", "input asset directory")
	out := fs.String("out", "", "new empty staging directory")
	arch := fs.String("arch", "", "native architecture")
	commit := fs.String("commit", "", "expected full source commit")
	tag := fs.String("tag", "", "tag must equal configured project.version")
	remote := fs.String("remote", "", "uploaded asset name/size JSON")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected arguments")
	}
	v, err := configuredVersion(".")
	if err != nil {
		return err
	}
	switch args[0] {
	case "version":
		if *tag != "" && *tag != v {
			return errors.New("tag does not equal configured project.version")
		}
		fmt.Println(v)
		return nil
	case "uploaded":
		err := uploaded(*dir, *remote, v)
		if err == nil {
			fmt.Println("[release-assets] all three public asset names/sizes PASS (not a remote content re-download)")
		}
		return err
	case "arch", "aggregate", "verify":
		if !shaRE.MatchString(*commit) {
			return errors.New("full lowercase source commit required")
		}
	default:
		return errors.New("unknown command")
	}
	switch args[0] {
	case "arch":
		if runtime.GOOS != "linux" || runtime.GOARCH != *arch {
			return errors.New("arch staging requires a matching native Linux Go host")
		}
		return prepareArch(*dir, *out, *arch, v, *commit)
	case "aggregate":
		if err := exactFiles(*dir, archFileNames(v)); err != nil {
			return err
		}
		m, err := collected(*dir, v, *commit)
		if err != nil {
			return err
		}
		if err = inspectAssets(*dir, m); err != nil {
			return err
		}
		if err = freshDir(*out); err != nil {
			return err
		}
		for _, n := range publicZipNames(v) {
			if err = copyFile(*dir, *out, n); err != nil {
				return err
			}
		}
		return writeComplete(*out, m)
	default:
		return verifyComplete(*dir, v, *commit)
	}
}
func nodeField(n *yaml.Node, k string) (*yaml.Node, error) {
	if n.Kind == yaml.DocumentNode && len(n.Content) == 1 {
		n = n.Content[0]
	}
	if n.Kind != yaml.MappingNode {
		return nil, errors.New("configuration mapping required")
	}
	var found *yaml.Node
	for i := 0; i < len(n.Content); i += 2 {
		if n.Content[i].Value == k {
			if found != nil {
				return nil, errors.New("duplicate configuration field")
			}
			found = n.Content[i+1]
		}
	}
	if found == nil {
		return nil, fmt.Errorf("missing configuration field %s", k)
	}
	return found, nil
}
func configuredVersion(root string) (string, error) {
	version := ""
	for _, f := range []string{"pre_build_config_template.yaml", "gen_pre_build_config_linux_amd64.yaml", "gen_pre_build_config_linux_arm64.yaml"} {
		b, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			return "", err
		}
		var n yaml.Node
		if err = yaml.Unmarshal(b, &n); err != nil {
			return "", err
		}
		p, err := nodeField(&n, "project")
		if err != nil {
			return "", err
		}
		v, err := nodeField(p, "version")
		if err != nil {
			return "", err
		}
		if v.Kind != yaml.ScalarNode || v.Tag != "!!str" || !versionRE.MatchString(v.Value) {
			return "", fmt.Errorf("invalid project.version in %s; require vX.Y.Z string", f)
		}
		if version != "" && version != v.Value {
			return "", errors.New("project.version differs across template/generated configs")
		}
		version = v.Value
	}
	return version, nil
}
func zipNames(v, a string) []string {
	ns := make([]string, 0, 3)
	for _, p := range products {
		ns = append(ns, p+"_linux_"+a+"_release_"+v+".zip")
	}
	sort.Strings(ns)
	return ns
}
func archManifest(a string) string { return "manifest-linux_" + a + ".json" }
func archSums(a string) string     { return "SHA256SUMS-linux_" + a + ".txt" }
func archFileNames(v string) []string {
	var ns []string
	for _, a := range arches {
		ns = append(ns, zipNames(v, a)...)
		ns = append(ns, archManifest(a), archSums(a))
	}
	sort.Strings(ns)
	return ns
}

// Public delivery is intentionally smaller than the internal six-package inventory.
func publicZipNames(v string) []string {
	var ns []string
	for _, a := range arches {
		ns = append(ns, "protenix_service_linux_"+a+"_release_"+v+".zip")
	}
	return ns
}
func completeNames(v string) []string { return append(publicZipNames(v), "SHA256SUMS") }
func freshDir(d string) error {
	if d == "" {
		return errors.New("empty output directory")
	}
	if err := os.MkdirAll(d, 0755); err != nil {
		return err
	}
	es, err := os.ReadDir(d)
	if err != nil {
		return err
	}
	if len(es) != 0 {
		return errors.New("output directory must be empty; refuse stale assets")
	}
	return nil
}
func exactFiles(d string, want []string) error {
	es, err := os.ReadDir(d)
	if err != nil {
		return err
	}
	got := make([]string, 0, len(es))
	for _, e := range es {
		if !e.Type().IsRegular() {
			return errors.New("asset directory must contain regular files only")
		}
		got = append(got, e.Name())
	}
	want = append([]string(nil), want...)
	sort.Strings(want)
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		return errors.New("incomplete or unexpected asset inventory")
	}
	return nil
}
func fileAsset(d, n string) (asset, error) {
	f, err := os.Open(filepath.Join(d, n))
	if err != nil {
		return asset{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return asset{}, err
	}
	if !st.Mode().IsRegular() {
		return asset{}, errors.New("asset must be regular file")
	}
	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return asset{}, err
	}
	return asset{n, size, hex.EncodeToString(h.Sum(nil))}, nil
}
func copyFile(src, dst, n string) error {
	in, err := os.Open(filepath.Join(src, n))
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(filepath.Join(dst, n), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, e := io.Copy(out, in)
	closeErr := out.Close()
	if e != nil {
		return e
	}
	return closeErr
}
func jsonBytes(m manifest) []byte { b, _ := json.MarshalIndent(m, "", "  "); return append(b, '\n') }
func writeJSON(d, n string, m manifest) error {
	return os.WriteFile(filepath.Join(d, n), jsonBytes(m), 0644)
}
func checksumBytes(d string, ns []string) ([]byte, error) {
	ns = append([]string(nil), ns...)
	sort.Strings(ns)
	var b strings.Builder
	for _, n := range ns {
		a, err := fileAsset(d, n)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&b, "%s  %s\n", a.SHA256, n)
	}
	return []byte(b.String()), nil
}
func writeSums(d, n string, ns []string) error {
	b, e := checksumBytes(d, ns)
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(d, n), b, 0644)
}
func readManifest(d, n string) (manifest, error) {
	b, err := os.ReadFile(filepath.Join(d, n))
	if err != nil {
		return manifest{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var m manifest
	if err = dec.Decode(&m); err != nil {
		return m, err
	}
	if dec.Decode(new(json.RawMessage)) != io.EOF {
		return m, errors.New("trailing manifest data")
	}
	return m, nil
}
func prepareArch(d, out, a, v, commit string) error {
	if a != "amd64" && a != "arm64" {
		return errors.New("unsupported architecture")
	}
	m := manifest{Schema: 1, Version: v, Commit: commit, Architecture: a, NativeHost: "linux/" + a}
	for _, n := range zipNames(v, a) {
		x, err := fileAsset(d, n)
		if err != nil {
			return err
		}
		m.Assets = append(m.Assets, x)
	}
	if err := inspectAssets(d, m); err != nil {
		return err
	}
	if err := freshDir(out); err != nil {
		return err
	}
	for _, x := range m.Assets {
		if err := copyFile(d, out, x.Name); err != nil {
			return err
		}
	}
	if err := writeJSON(out, archManifest(a), m); err != nil {
		return err
	}
	if err := writeSums(out, archSums(a), append(zipNames(v, a), archManifest(a))); err != nil {
		return err
	}
	fmt.Printf("[release-assets] staged three native linux/%s packages, manifest and checksums\n", a)
	return nil
}
func collected(d, v, commit string) (manifest, error) {
	all := manifest{Schema: 1, Version: v, Commit: commit, Architecture: "amd64+arm64"}
	for _, a := range arches {
		m, err := readManifest(d, archManifest(a))
		if err != nil {
			return all, err
		}
		if m.Schema != 1 || m.Version != v || m.Commit != commit || m.Architecture != a || m.NativeHost != "linux/"+a {
			return all, errors.New("architecture manifest metadata mismatch")
		}
		var expected []asset
		for _, n := range zipNames(v, a) {
			x, err := fileAsset(d, n)
			if err != nil {
				return all, err
			}
			expected = append(expected, x)
		}
		if !reflect.DeepEqual(m.Assets, expected) {
			return all, errors.New("package inventory, size or checksum mismatch")
		}
		sums, err := checksumBytes(d, append(zipNames(v, a), archManifest(a)))
		if err != nil {
			return all, err
		}
		actual, err := os.ReadFile(filepath.Join(d, archSums(a)))
		if err != nil {
			return all, err
		}
		if !bytes.Equal(sums, actual) {
			return all, errors.New("architecture checksum file mismatch")
		}
		all.Assets = append(all.Assets, m.Assets...)
	}
	sort.Slice(all.Assets, func(i, j int) bool { return all.Assets[i].Name < all.Assets[j].Name })
	return all, nil
}
func inspectAssets(d string, m manifest) error {
	for _, x := range m.Assets {
		a := ""
		p := ""
		for _, arch := range arches {
			for _, product := range products {
				if x.Name == product+"_linux_"+arch+"_release_"+m.Version+".zip" {
					a = arch
					p = product
				}
			}
		}
		if p == "" {
			return errors.New("unexpected package name")
		}
		if err := inspectZip(filepath.Join(d, x.Name), p, a, m.Version, m.Commit); err != nil {
			return fmt.Errorf("%s: %w", x.Name, err)
		}
	}
	return nil
}
func readEntry(f *zip.File, limit int64) ([]byte, error) {
	if f == nil {
		return nil, errors.New("required package member missing")
	}
	if f.UncompressedSize64 > uint64(limit) {
		return nil, errors.New("package member too large")
	}
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if int64(len(b)) > limit {
		return nil, errors.New("package member exceeds limit")
	}
	return b, err
}
func checkBI(b []byte, p, a, v, c string) error {
	fields := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		k, val, ok := strings.Cut(line, "=")
		if !ok {
			return errors.New("invalid BUILD_INFO record")
		}
		if _, ok = fields[k]; ok {
			return errors.New("duplicate BUILD_INFO key")
		}
		fields[k] = val
	}
	for k, want := range map[string]string{"product": p, "version": v, "goos": "linux", "goarch": a, "run_mode": "release", "commit": c, "commit_head": c, "dirty": "0"} {
		if fields[k] != want {
			return fmt.Errorf("BUILD_INFO mismatch: %s", k)
		}
	}
	return nil
}
func checkBinary(b []byte, a string) error {
	f, err := elf.NewFile(bytes.NewReader(b))
	if err != nil {
		return errors.New("service binary is not ELF")
	}
	defer f.Close()
	want := elf.EM_X86_64
	if a == "arm64" {
		want = elf.EM_AARCH64
	}
	if f.Class != elf.ELFCLASS64 || f.Machine != want {
		return errors.New("service ELF architecture mismatch")
	}
	bi, err := buildinfo.Read(bytes.NewReader(b))
	if err != nil {
		return errors.New("service Go build info missing")
	}
	settings := map[string]string{}
	for _, s := range bi.Settings {
		settings[s.Key] = s.Value
	}
	if settings["GOOS"] != "linux" || settings["GOARCH"] != a || settings["CGO_ENABLED"] != "1" {
		return errors.New("service requires matching linux architecture and CGo build info")
	}
	return nil
}
func inspectZip(file, p, a, v, c string) error {
	z, err := zip.OpenReader(file)
	if err != nil {
		return err
	}
	defer z.Close()
	members := map[string]*zip.File{}
	for _, f := range z.File {
		n := strings.TrimSuffix(f.Name, "/")
		if path.Clean(n) != n || !strings.HasPrefix(n, p+"/") && n != p {
			return errors.New("unsafe package member path")
		}
		if _, ok := members[f.Name]; ok {
			return errors.New("duplicate package member")
		}
		members[f.Name] = f
	}
	root := p + "/"
	bi, err := readEntry(members[root+"BUILD_INFO.txt"], 1<<20)
	if err != nil {
		return err
	}
	if err = checkBI(bi, p, a, v, c); err != nil {
		return err
	}
	req := []string{root + "install_" + p + ".sh"}
	if p == "protenix_service" {
		req = append(req, root+p+".service", root+"conf/config.yaml")
		for _, runtime := range []string{"protenix", "pxdesign"} {
			r := root + "runtime/" + runtime + "/"
			req = append(req, r+"install_"+runtime+".sh", r+"src/"+runtime+"/setup.py")
			b, e := readEntry(members[r+"BUILD_INFO.txt"], 1<<20)
			if e != nil {
				return e
			}
			if e = checkBI(b, runtime, a, v, c); e != nil {
				return e
			}
		}
		bin, e := readEntry(members[root+p], 512<<20)
		if e != nil {
			return e
		}
		if e = checkBinary(bin, a); e != nil {
			return e
		}
	} else {
		req = append(req, root+"bin/"+p+"-run", root+"scripts/install_prefix_"+p+".sh", root+"deps/"+p+".txt", root+"src/"+p+"/setup.py")
	}
	for _, n := range req {
		f := members[n]
		if f == nil || f.FileInfo().IsDir() || f.UncompressedSize64 == 0 {
			return errors.New("required nonempty package material missing")
		}
	}
	return nil
}
func writeComplete(d string, m manifest) error {
	return writeSums(d, "SHA256SUMS", publicZipNames(m.Version))
}
func verifyComplete(d, v, c string) error {
	if err := exactFiles(d, completeNames(v)); err != nil {
		return err
	}
	for _, a := range arches {
		n := "protenix_service_linux_" + a + "_release_" + v + ".zip"
		if err := inspectZip(filepath.Join(d, n), "protenix_service", a, v, c); err != nil {
			return err
		}
	}
	sums, err := checksumBytes(d, publicZipNames(v))
	if err != nil {
		return err
	}
	actual, err := os.ReadFile(filepath.Join(d, "SHA256SUMS"))
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, sums) {
		return errors.New("public release checksums mismatch")
	}
	fmt.Println("[release-assets] two service packages, source commit, ELF/CGo and checksums PASS")
	return nil
}
func uploaded(d, remote, v string) error {
	if remote == "" {
		return errors.New("uploaded inventory required")
	}
	if err := exactFiles(d, completeNames(v)); err != nil {
		return err
	}
	type remoteAsset struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}
	b, err := os.ReadFile(remote)
	if err != nil {
		return err
	}
	var got []remoteAsset
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&got); err != nil {
		return err
	}
	if dec.Decode(new(json.RawMessage)) != io.EOF {
		return errors.New("trailing uploaded inventory data")
	}
	var want []remoteAsset
	for _, n := range completeNames(v) {
		a, err := fileAsset(d, n)
		if err != nil {
			return err
		}
		want = append(want, remoteAsset{n, a.Bytes})
	}
	sort.Slice(got, func(i, j int) bool { return got[i].Name < got[j].Name })
	sort.Slice(want, func(i, j int) bool { return want[i].Name < want[j].Name })
	if !reflect.DeepEqual(got, want) {
		return errors.New("uploaded public release asset names/counts/sizes mismatch")
	}
	return nil
}

// These are explicit fixture/static checks, NOT Actions/native runtime/GPU evidence.
func selfTest() error {
	if err := os.MkdirAll("tmp", 0755); err != nil {
		return err
	}
	root, err := os.MkdirTemp("tmp", "release-contract-check-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	checks := 0
	assert := func(name string, ok bool) error {
		checks++
		if !ok {
			return fmt.Errorf("self-test failed: %s", name)
		}
		return nil
	}
	must := func(err error) {
		if err != nil {
			panic(err)
		}
	}
	v := "v0.0.1"
	c := strings.Repeat("a", 40)
	bi := func(p, a string) []byte {
		return []byte(fmt.Sprintf("product=%s\nversion=%s\ngoos=linux\ngoarch=%s\nrun_mode=release\ncommit=%s\ncommit_head=%s\ndirty=0\n", p, v, a, c, c))
	}
	for _, tc := range []struct {
		name, old, new string
		valid          bool
	}{
		{"valid", "", "", true}, {"dirty", "dirty=0", "dirty=1", false}, {"wrong-version", v, "v9.9.9", false},
		{"wrong-commit", c, strings.Repeat("b", 40), false}, {"wrong-arch", "goarch=arm64", "goarch=amd64", false},
		{"wrong-product", "product=protenix", "product=pxdesign", false}, {"wrong-mode", "run_mode=release", "run_mode=debug", false},
	} {
		b := bi("protenix", "arm64")
		if tc.old != "" {
			b = []byte(strings.ReplaceAll(string(b), tc.old, tc.new))
		}
		if err = assert("BUILD_INFO/"+tc.name, (checkBI(b, "protenix", "arm64", v, c) == nil) == tc.valid); err != nil {
			return err
		}
	}
	if err = assert("BUILD_INFO/duplicate", checkBI(append(bi("protenix", "arm64"), []byte("dirty=0\n")...), "protenix", "arm64", v, c) != nil); err != nil {
		return err
	}
	for _, tc := range []struct {
		name, raw string
		valid     bool
	}{
		{"string", "v0.0.1", true}, {"numeric", "1.2", false}, {"boolean", "true", false}, {"mapping", "{}", false}, {"sequence", "[]", false},
		{"malformed", "v1", false}, {"leading-zero", "v01.2.3", false}, {"duplicate", "v0.0.1\n  version: v0.0.1", false},
	} {
		d := filepath.Join(root, "version-"+tc.name)
		must(os.MkdirAll(d, 0755))
		for _, f := range []string{"pre_build_config_template.yaml", "gen_pre_build_config_linux_amd64.yaml", "gen_pre_build_config_linux_arm64.yaml"} {
			must(os.WriteFile(filepath.Join(d, f), []byte("project:\n  version: "+tc.raw+"\n"), 0644))
		}
		_, e := configuredVersion(d)
		if err = assert("version/"+tc.name, (e == nil) == tc.valid); err != nil {
			return err
		}
		if tc.valid {
			must(os.WriteFile(filepath.Join(d, "gen_pre_build_config_linux_arm64.yaml"), []byte("project:\n  version: v0.0.2\n"), 0644))
			_, e = configuredVersion(d)
			if err = assert("version/cross-config-mismatch", e != nil); err != nil {
				return err
			}
		}
	}
	// Two-architecture manifest fixtures intentionally contain no inference code or ELF.
	// collected checks metadata/hashes only; inspectAssets must reject this fake set.
	d := filepath.Join(root, "inventory")
	must(os.MkdirAll(d, 0755))
	for _, a := range arches {
		m := manifest{Schema: 1, Version: v, Commit: c, Architecture: a, NativeHost: "linux/" + a}
		for _, n := range zipNames(v, a) {
			must(os.WriteFile(filepath.Join(d, n), []byte("manifest-contract-fixture-not-a-package"), 0644))
			x, e := fileAsset(d, n)
			must(e)
			m.Assets = append(m.Assets, x)
		}
		must(writeJSON(d, archManifest(a), m))
		must(writeSums(d, archSums(a), append(zipNames(v, a), archManifest(a))))
	}
	m, e := collected(d, v, c)
	if err = assert("manifest/complete-fixture", e == nil && len(m.Assets) == 6); err != nil {
		return err
	}
	if err = assert("package/fake-six-packages-rejected", inspectAssets(d, m) != nil); err != nil {
		return err
	}
	if err = assert("manifest/wrong-source-commit", func() bool { _, e := collected(d, v, strings.Repeat("b", 40)); return e != nil }()); err != nil {
		return err
	}
	n := zipNames(v, "amd64")[0]
	original, e := os.ReadFile(filepath.Join(d, n))
	must(e)
	must(os.WriteFile(filepath.Join(d, n), []byte("tampered"), 0644))
	_, e = collected(d, v, c)
	if err = assert("manifest/tampered-package", e != nil); err != nil {
		return err
	}
	must(os.WriteFile(filepath.Join(d, n), original, 0644))
	am, e := os.ReadFile(filepath.Join(d, archManifest("arm64")))
	must(e)
	must(os.Remove(filepath.Join(d, archManifest("arm64"))))
	_, e = collected(d, v, c)
	if err = assert("manifest/missing-architecture", e != nil); err != nil {
		return err
	}
	must(os.WriteFile(filepath.Join(d, archManifest("arm64")), am, 0644))
	for _, tc := range []string{"wrong-native-host", "wrong-arch", "duplicate-asset", "missing-asset", "unknown-schema"} {
		bad, e := readManifest(d, archManifest("arm64"))
		must(e)
		switch tc {
		case "wrong-native-host":
			bad.NativeHost = "linux/amd64"
		case "wrong-arch":
			bad.Architecture = "amd64"
		case "duplicate-asset":
			bad.Assets = append(bad.Assets, bad.Assets[0])
		case "missing-asset":
			bad.Assets = bad.Assets[1:]
		case "unknown-schema":
			bad.Schema = 2
		}
		must(writeJSON(d, archManifest("arm64"), bad))
		_, e = collected(d, v, c)
		if err = assert("manifest/"+tc, e != nil); err != nil {
			return err
		}
		must(os.WriteFile(filepath.Join(d, archManifest("arm64")), am, 0644))
	}
	must(os.WriteFile(filepath.Join(d, "unexpected.zip"), []byte("fixture"), 0644))
	if err = assert("inventory/extra-file", exactFiles(d, archFileNames(v)) != nil); err != nil {
		return err
	}
	must(os.Remove(filepath.Join(d, "unexpected.zip")))
	internalDir := d
	d = filepath.Join(root, "public")
	must(freshDir(d))
	for _, n := range publicZipNames(v) {
		must(copyFile(internalDir, d, n))
	}
	must(writeComplete(d, m))
	if err = assert("inventory/three-public-files", exactFiles(d, completeNames(v)) == nil); err != nil {
		return err
	}
	wantPublic := []string{"protenix_service_linux_amd64_release_" + v + ".zip", "protenix_service_linux_arm64_release_" + v + ".zip", "SHA256SUMS"}
	if err = assert("public/exact-delivery-contract", reflect.DeepEqual(completeNames(v), wantPublic)); err != nil {
		return err
	}
	sums, sumErr := os.ReadFile(filepath.Join(d, "SHA256SUMS"))
	if err = assert("public/two-checksum-lines", sumErr == nil && len(strings.Split(strings.TrimSpace(string(sums)), "\n")) == 2 && !bytes.Contains(sums, []byte("manifest"))); err != nil {
		return err
	}
	if err = assert("public/fake-packages-rejected", verifyComplete(d, v, c) != nil); err != nil {
		return err
	}
	must(os.WriteFile(filepath.Join(d, "release-manifest.json"), []byte("stale"), 0644))
	if err = assert("public/stale-file-rejected", exactFiles(d, completeNames(v)) != nil); err != nil {
		return err
	}
	must(os.Remove(filepath.Join(d, "release-manifest.json")))
	if err = assert("staging/nonempty-refused", freshDir(d) != nil); err != nil {
		return err
	}
	type remoteAsset struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}
	var remote []remoteAsset
	for _, n := range completeNames(v) {
		x, e := fileAsset(d, n)
		must(e)
		remote = append(remote, remoteAsset{n, x.Bytes})
	}
	remotePath := filepath.Join(root, "remote.json")
	for _, tc := range []string{"complete", "missing", "extra", "size", "duplicate"} {
		rs := append([]remoteAsset(nil), remote...)
		switch tc {
		case "missing":
			rs = rs[1:]
		case "extra":
			rs = append(rs, remoteAsset{"unexpected", 1})
		case "size":
			rs[0].Size++
		case "duplicate":
			rs[0] = rs[1]
		}
		b, e := json.Marshal(rs)
		must(e)
		must(os.WriteFile(remotePath, b, 0644))
		e = uploaded(d, remotePath, v)
		if err = assert("upload/"+tc, (e == nil) == (tc == "complete")); err != nil {
			return err
		}
	}
	// Standalone archive parser fixtures: real ZIP format, deliberately not runtime acceptance.
	zipPath := filepath.Join(root, "standalone.zip")
	for _, tc := range []string{"complete", "missing-setup", "wrong-arch", "duplicate-member", "traversal"} {
		f, e := os.Create(zipPath)
		must(e)
		w := zip.NewWriter(f)
		members := map[string][]byte{"protenix/BUILD_INFO.txt": bi("protenix", "arm64"), "protenix/install_protenix.sh": []byte("fixture"), "protenix/bin/protenix-run": []byte("fixture"), "protenix/scripts/install_prefix_protenix.sh": []byte("fixture"), "protenix/deps/protenix.txt": []byte("fixture"), "protenix/src/protenix/setup.py": []byte("fixture")}
		if tc == "missing-setup" {
			delete(members, "protenix/src/protenix/setup.py")
		}
		if tc == "wrong-arch" {
			members["protenix/BUILD_INFO.txt"] = bi("protenix", "amd64")
		}
		if tc == "traversal" {
			members["protenix/../outside"] = []byte("fixture")
		}
		for n, b := range members {
			fw, e := w.Create(n)
			must(e)
			_, e = fw.Write(b)
			must(e)
		}
		if tc == "duplicate-member" {
			fw, e := w.Create("protenix/BUILD_INFO.txt")
			must(e)
			_, e = fw.Write(bi("protenix", "arm64"))
			must(e)
		}
		must(w.Close())
		must(f.Close())
		e = inspectZip(zipPath, "protenix", "arm64", v, c)
		if err = assert("zip/"+tc, (e == nil) == (tc == "complete")); err != nil {
			return err
		}
	}
	if err = assert("binary/non-ELF-rejected", checkBinary([]byte("fixture"), "arm64") != nil); err != nil {
		return err
	}
	self, e := os.Executable()
	must(e)
	binary, e := os.ReadFile(self)
	must(e)
	if runtime.GOOS == "linux" && (runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64") {
		// Parser test on this helper's ELF; not a claim that it is the service or uses SQLite.
		wrong := "arm64"
		if runtime.GOARCH == "arm64" {
			wrong = "amd64"
		}
		if err = assert("binary/wrong-ELF-arch-rejected", checkBinary(binary, wrong) != nil); err != nil {
			return err
		}
	}
	if err = checkWorkflow(); err != nil {
		return err
	}
	checks++
	if err = checkGitTagEntry(); err != nil {
		return err
	}
	checks++
	for _, script := range []string{"git_tag.sh", "build.sh", "scripts/check_native_build.sh"} {
		if e := exec.Command("bash", "-n", script).Run(); e != nil {
			return fmt.Errorf("shell syntax: %s", script)
		}
		checks++
	}
	if err = checkNativeFixtures(root); err != nil {
		return err
	}
	checks += 5
	buildScript, readErr := os.ReadFile("build.sh")
	if err = assert("build/full-main-package", readErr == nil && strings.Contains(string(buildScript), `go build -o "${out}" -trimpath -ldflags "${ld}" . )`)); err != nil {
		return err
	}
	fmt.Printf("[release-assets] %d fixture/static contract checks PASS; NOT an Actions run or GPU/install acceptance\n", checks)
	return nil
}
func checkWorkflow() error {
	b, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		return err
	}
	type step struct {
		If   string            `yaml:"if"`
		Uses string            `yaml:"uses"`
		Run  string            `yaml:"run"`
		With map[string]string `yaml:"with"`
	}
	type job struct {
		If       string `yaml:"if"`
		Needs    string `yaml:"needs"`
		RunsOn   string `yaml:"runs-on"`
		Strategy struct {
			FailFast bool `yaml:"fail-fast"`
			Matrix   struct {
				Include []struct {
					Arch   string `yaml:"arch"`
					Runner string `yaml:"runner"`
				} `yaml:"include"`
			} `yaml:"matrix"`
		} `yaml:"strategy"`
		Steps []step `yaml:"steps"`
	}
	var wf struct {
		On   map[string]yaml.Node `yaml:"on"`
		Jobs map[string]job       `yaml:"jobs"`
	}
	if err = yaml.Unmarshal(b, &wf); err != nil {
		return err
	}
	release := wf.Jobs["release"]
	native := wf.Jobs["build_native"]
	build := wf.Jobs["Build"]
	prune := wf.Jobs["RemoveOldRelease"]
	tagOnly := "github.event_name == 'push' && startsWith(github.ref, 'refs/tags/v')"
	candidateOnly := "github.event_name == 'workflow_dispatch' || github.ref == 'refs/heads/main'"
	if build.If != "always() && !cancelled() && needs.build_native.result == 'success'" || release.If != tagOnly || prune.If != tagOnly || !strings.Contains(native.If, "always()") || !strings.Contains(native.If, "github.ref == 'refs/heads/main'") {
		return errors.New("candidate must bypass release/prune without bypassing native build")
	}
	foundCandidate := false
	for _, st := range build.Steps {
		if strings.HasPrefix(st.Uses, "george012/upload_github_release_assets_action@") && st.If != tagOnly {
			return errors.New("candidate must not upload to a Release")
		}
		if st.Uses == "actions/upload-artifact@v7" && st.With["name"] == "candidate-${{ github.sha }}" {
			foundCandidate = st.If == candidateOnly && st.With["path"] == "tmp/release-assets/*"
		}
	}
	if !foundCandidate {
		return errors.New("candidate public-three-file artifact contract missing")
	}
	if len(wf.Jobs) != 4 || release.Needs != "" || native.Needs != "release" || build.Needs != "build_native" || prune.Needs != "Build" {
		return errors.New("workflow must keep release → Build → RemoveOldRelease, with native packaging inside Build")
	}
	if release.RunsOn != "ubuntu-latest" || build.RunsOn != "ubuntu-latest" || prune.RunsOn != "ubuntu-latest" {
		return errors.New("template stage runners mismatch")
	}
	if native.RunsOn != "${{ matrix.runner }}" || native.Strategy.FailFast || len(native.Strategy.Matrix.Include) != 2 {
		return errors.New("native matrix contract mismatch")
	}
	want := map[string]string{"amd64": "ubuntu-24.04", "arm64": "ubuntu-24.04-arm"}
	for _, m := range native.Strategy.Matrix.Include {
		if want[m.Arch] != m.Runner {
			return errors.New("workflow runner mismatch")
		}
		delete(want, m.Arch)
	}
	if len(want) != 0 {
		return errors.New("workflow runner mismatch")
	}
	if _, ok := wf.On["workflow_dispatch"]; !ok {
		return errors.New("missing workflow_dispatch")
	}
	if _, ok := wf.On["push"]; !ok {
		return errors.New("missing tag trigger")
	}
	for name, j := range wf.Jobs {
		for _, s := range j.Steps {
			if strings.HasPrefix(s.Uses, "george012/upload_github_release_assets_action@") && s.With["path"] != "./tmp/release-assets/*" {
				return errors.New("upload must use the fresh verified public-only staging directory")
			}
			if s.Uses == "actions/setup-go@v6" && s.With["go-version-file"] != "protenix_service/go.mod" {
				return errors.New("CI must provision the actual service Go toolchain")
			}
			if s.Run != "" {
				cmd := exec.Command("bash", "-n")
				cmd.Stdin = strings.NewReader(s.Run)
				if err = cmd.Run(); err != nil {
					return fmt.Errorf("workflow run shell syntax in %s", name)
				}
			}
		}
	}
	text := string(b)
	for _, required := range []string{
		"mathieudutour/github-tag-action@v6.2",
		"ncipollo/release-action@v1",
		"bodyFile: changelog/${{ env.LATEST_TAG }}.md",
		"bash ./build.sh pack release",
		"check_native_build.sh",
		"gen_release_assets.go aggregate",
		"gen_release_assets.go verify",
		"name: release-packages-linux-${{ matrix.arch }}",
		"pattern: release-packages-linux-*",
		"merge-multiple: true",
		"george012/upload_github_release_assets_action@latest",
		"release_remove_old_if_present \"$APP_OLD_VERSION\" \"$GITHUB_REPOSITORY\"",
		"v0.0.2 is preserved; no-op",
	} {
		if !strings.Contains(text, required) {
			return errors.New("workflow template or necessary packaging contract missing")
		}
	}
	if strings.Contains(text, "--clobber") || strings.Contains(text, "gcc-aarch64-linux-gnu") || strings.Contains(text, "gh release create") || strings.Contains(text, "--draft") {
		return errors.New("workflow must not reintroduce cross-compile or the previous draft-publish redesign")
	}
	if strings.Contains(text, "bash ./build.sh test") || strings.Contains(text, "bash ./build.sh\n") {
		return errors.New("Actions must not run default build.sh all or a second test-mode pack")
	}
	return nil
}

func checkGitTagEntry() error {
	b, err := os.ReadFile("git_tag.sh")
	if err != nil {
		return err
	}
	text := string(b)
	for _, required := range []string{
		`if [ -z "$1" ]; then`,
		`NEXT_VERSION="v${base}.${major}.${minor}"`,
		`elif [ "$1" == "custom" ]; then`,
		`git tag v${next_version_no}`,
		`git tag -f latest v${next_version_no}`,
		`for remote in $(git remote)`,
		`git push --delete ${remote} latest`,
		`git push ${remote} v${next_version_no}`,
		`release_delete_local_tag_if_present "v${pre_del_version_no}"`,
		`handle_input "$@"`,
		`./protenix_service/gen_api_docs.sh "${NEXT_VERSION}"`,
		`pre_build_config_template.yaml`,
		`bash ./gen_pre_build_config.sh all`,
	} {
		if !strings.Contains(text, required) {
			return errors.New("git_tag.sh lost a template or necessary path adaptation")
		}
	}
	for _, forbidden := range []string{"--publish", "--check", "git tag -a"} {
		if strings.Contains(text, forbidden) {
			return errors.New("git_tag.sh must not keep the unauthorized flag/annotated-tag interface")
		}
	}
	return checkGitTagNoArgFixture()
}

func checkGitTagNoArgFixture() error {
	// Isolated git repository only. This does not tag or push the product checkout.
	base, err := os.MkdirTemp("tmp", "git-tag-contract-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(base)
	root, err := filepath.Abs(base)
	if err != nil {
		return err
	}
	repo := filepath.Join(root, "work")
	remote := filepath.Join(root, "remote.git")
	for _, d := range []string{repo, filepath.Join(repo, "protenix_service", "config"), filepath.Join(repo, "protenix_service", "docs"), filepath.Join(repo, "scripts"), filepath.Join(repo, "bin")} {
		if err = os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	write := func(path, body string, mode os.FileMode) {
		if err = os.WriteFile(filepath.Join(repo, path), []byte(body), mode); err != nil {
			panic(err)
		}
	}
	src, err := os.ReadFile("git_tag.sh")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(repo, "git_tag.sh"), src, 0755); err != nil {
		return err
	}
	cleanupSource, err := os.ReadFile("scripts/release_cleanup.sh")
	if err != nil {
		return err
	}
	write("scripts/release_cleanup.sh", string(cleanupSource), 0644)
	write("pre_build_config_template.yaml", "project:\n  name: protenix_build\n  version: v0.0.1\n", 0644)
	write("gen_pre_build_config_linux_amd64.yaml", "project:\n  version: v0.0.1\n", 0644)
	write("gen_pre_build_config_linux_arm64.yaml", "project:\n  version: v0.0.1\n", 0644)
	write("protenix_service/config/config.go", "package config\nconst (\n\tProjectVersion  = \"v0.0.1\"\n)\n", 0644)
	write("gen_pre_build_config.sh", "#!/bin/bash\nset -e\nsed -i -e 's/^\\(  version: \\)v0.0.1$/\\1v0.0.2/' gen_pre_build_config_linux_amd64.yaml gen_pre_build_config_linux_arm64.yaml\n", 0755)
	write("protenix_service/gen_api_docs.sh", "#!/bin/bash\nset -e\nprintf 'methods for %s\\n' \"$1\" > protenix_service/docs/api_methods.md\n", 0755)
	write("bin/go", `#!/bin/bash
set -euo pipefail
if [[ "$1" != run || "$2" != gen_release_assets.go ]]; then
  echo "unexpected go invocation: $*" >&2
  exit 1
fi
shift 2
case "$1" in
  version)
    current=$(sed -n 's/^  version: //p' pre_build_config_template.yaml | head -n1)
    if [[ "${2:-}" == --tag=* ]]; then
      tag="${2#--tag=}"
      test "$current" = "$tag"
      exit 0
    fi
    printf '%s\n' "$current"
    ;;
  *)
    echo "unexpected gen_release_assets command: $*" >&2
    exit 1
    ;;
esac
`, 0755)
	run := func(dir string, extraEnv []string, args ...string) ([]byte, error) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(append([]string{}, os.Environ()...), extraEnv...)
		return cmd.CombinedOutput()
	}
	if _, err = run(root, nil, "git", "init", "--bare", remote); err != nil {
		return err
	}
	setup := [][]string{
		{"git", "init", "-b", "main", repo},
		{"git", "-C", repo, "config", "user.name", "yerikokay"},
		{"git", "-C", repo, "config", "user.email", "yerikokay@users.noreply.github.com"},
		{"git", "-C", repo, "add", "."},
		{"git", "-C", repo, "commit", "-m", "fixture base"},
		{"git", "-C", repo, "tag", "v0.0.0"},
		{"git", "-C", repo, "tag", "latest"},
		{"git", "-C", repo, "remote", "add", "origin", remote},
		{"git", "-C", repo, "push", "-u", "origin", "HEAD:main", "v0.0.0", "latest"},
	}
	for _, args := range setup {
		if out, e := run("", nil, args...); e != nil {
			return fmt.Errorf("fixture setup failed: %s: %s: %w", strings.Join(args, " "), out, e)
		}
	}
	pathEnv := "PATH=" + filepath.Join(repo, "bin") + string(os.PathListSeparator) + os.Getenv("PATH")
	out, err := run(repo, []string{pathEnv}, "bash", "./git_tag.sh", "--publish", "v0.0.1")
	if err != nil {
		return fmt.Errorf("unexpected failure on invalid flag: %s", out)
	}
	if !strings.Contains(string(out), "Invalid argument") {
		return errors.New("unauthorized --publish flag must fall through to the template invalid-argument path")
	}
	if out, err = run(repo, nil, "git", "rev-parse", "HEAD"); err != nil {
		return err
	}
	before := strings.TrimSpace(string(out))
	if out, err = run(repo, []string{pathEnv}, "bash", "./git_tag.sh", "-get_pre_del_tag_name"); err != nil {
		return fmt.Errorf("pre-delete name failed: %s", out)
	}
	if !strings.Contains(string(out), "0.0.0") {
		return fmt.Errorf("pre-delete name mismatch: %s", out)
	}
	out, err = run(repo, []string{pathEnv}, "bash", "./git_tag.sh")
	if err != nil {
		return fmt.Errorf("no-arg git_tag.sh fixture failed: %s", out)
	}
	if !strings.Contains(string(out), "Complated") {
		return fmt.Errorf("no-arg flow did not complete: %s", out)
	}
	checks := map[string][]string{
		"service-version": {"grep", "-q", `ProjectVersion  = "v0.0.2"`, "protenix_service/config/config.go"},
		"yaml-version":    {"grep", "-qx", "  version: v0.0.2", "pre_build_config_template.yaml", "gen_pre_build_config_linux_amd64.yaml", "gen_pre_build_config_linux_arm64.yaml"},
		"lightweight-tag": {"bash", "-c", "test $(git cat-file -t v0.0.2) = commit"},
		"latest-move":     {"bash", "-c", "test $(git rev-parse latest) = $(git rev-parse v0.0.2)"},
		"old-local-tag":   {"bash", "-c", "! git show-ref --verify --quiet refs/tags/v0.0.0"},
		"release-commit":  {"bash", "-c", "git log -1 --format=%s | grep -q '^Release:--: v0.0.2_'"},
		"changelog":       {"test", "-s", "changelog/v0.0.2.md"},
		"api-docs":        {"test", "-s", "protenix_service/docs/api_methods.md"},
	}
	for name, args := range checks {
		if _, e := run(repo, nil, args...); e != nil {
			return fmt.Errorf("no-arg result failed: %s", name)
		}
	}
	if out, err = run(repo, nil, "git", "rev-parse", "HEAD"); err != nil {
		return err
	}
	if strings.TrimSpace(string(out)) == before {
		return errors.New("no-arg flow did not create the template release commit")
	}
	// Confirm the fixture remote received main and the version tag, and that latest was replaced.
	if _, err = run(remote, nil, "git", "rev-parse", "refs/heads/main", "refs/tags/v0.0.2", "refs/tags/latest"); err != nil {
		return errors.New("fixture remote did not receive the template main/tag/latest pushes")
	}
	return nil
}

func checkNativeFixtures(root string) error {
	// Stub executables exercise preflight branches only; no install and no real compiler run.
	dir := filepath.Join(root, "native-bin")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	stubs := map[string]string{
		"uname": "if [[ $1 == -s ]]; then echo Linux; else echo \"${FIX_HOST:-aarch64}\"; fi",
		"go":    "case $2 in GOHOSTOS|GOOS) echo linux ;; GOHOSTARCH) echo arm64 ;; GOARCH) echo \"${FIX_GOARCH:-arm64}\" ;; CGO_ENABLED) echo \"${FIX_CGO:-1}\" ;; *) exit 1 ;; esac",
		"gcc":   "echo \"${FIX_GCC:-aarch64-linux-gnu}\"", "bun": ":", "zip": ":", "unzip": ":",
	}
	for n, s := range stubs {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("#!/bin/bash\nset -eu\n"+s+"\n"), 0755); err != nil {
			return err
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	for _, tc := range []struct {
		name, env string
		pass      bool
	}{{"native", "FIX_CGO=1", true}, {"host", "FIX_HOST=x86_64", false}, {"goarch", "FIX_GOARCH=amd64", false}, {"cgo", "FIX_CGO=0", false}, {"compiler", "FIX_GCC=x86_64-linux-gnu", false}} {
		cmd := exec.Command("/bin/bash", "scripts/check_native_build.sh", "arm64")
		cmd.Env = []string{"PATH=" + abs, tc.env}
		e := cmd.Run()
		if (e == nil) != tc.pass {
			return fmt.Errorf("native preflight fixture failed: %s", tc.name)
		}
	}
	return nil
}
