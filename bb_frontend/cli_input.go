package bb_frontend

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_config"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_session"
	"github.com/0xdevelop/bioBox/bb_config"
	"github.com/google/uuid"
)

const cliInputMaxBytes = 32 << 20

// receiveCLIInput 把浏览器交来的输入文件写到本机，返回已注册方法 input 要用的路径。
func receiveCLIInput(writer http.ResponseWriter, request *http.Request) {
	if !cliInputAllowed(request) {
		writer.WriteHeader(http.StatusForbidden)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, cliInputMaxBytes)
	if err := request.ParseMultipartForm(cliInputMaxBytes); err != nil {
		http.Error(writer, "input file is invalid", http.StatusBadRequest)
		return
	}
	suite := strings.TrimSpace(request.FormValue("suite"))
	if suite != "protenix" && suite != "pxdesign" {
		http.Error(writer, "suite is invalid", http.StatusBadRequest)
		return
	}
	file, header, err := request.FormFile("file")
	if err != nil {
		http.Error(writer, "input file is missing", http.StatusBadRequest)
		return
	}
	defer file.Close()
	body, err := io.ReadAll(file)
	if err != nil || len(body) == 0 {
		http.Error(writer, "input file is empty", http.StatusBadRequest)
		return
	}
	landed, err := landCLIInput(suite, header.Filename, body)
	if err != nil {
		http.Error(writer, "input file was not stored", http.StatusBadRequest)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(map[string]string{"path": landed})
}

func cliInputAllowed(request *http.Request) bool {
	if !bb_api_auth_config.AuthGateEnabled() {
		return true
	}
	ctx := bb_api_auth_session.WithBearerToken(request.Context(), request.Header.Get("Authorization"))
	_, err := bb_api_auth_session.AuthenticateRequest(ctx, map[string]interface{}{})
	return err == nil
}

func landCLIInput(suite, originalName string, body []byte) (string, error) {
	name := filepath.Base(strings.TrimSpace(originalName))
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return "", os.ErrInvalid
	}
	dir := cliInputDir(suite)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(dir, uuid.NewString()+"-"+name)
	if err := os.WriteFile(target, body, 0o644); err != nil {
		return "", err
	}
	return filepath.Abs(target)
}

func cliInputDir(suite string) string {
	if suite == "pxdesign" {
		if current := runtimeCurrentDir(suite); current != "" {
			return current
		}
	}
	root := "data/frontend_inputs"
	if bb_config.GlobalConfig != nil && bb_config.GlobalConfig.CliCfg != nil {
		if workRoot := strings.TrimSpace(bb_config.GlobalConfig.CliCfg.WorkRoot); workRoot != "" {
			root = filepath.Join(workRoot, "frontend_inputs")
		}
	}
	return filepath.Join(root, suite)
}

func runtimeCurrentDir(suite string) string {
	if bb_config.GlobalConfig == nil || bb_config.GlobalConfig.CliCfg == nil {
		return ""
	}
	var binary string
	switch suite {
	case "protenix":
		if bb_config.GlobalConfig.CliCfg.Protenix != nil {
			binary = bb_config.GlobalConfig.CliCfg.Protenix.Binary
		}
	case "pxdesign":
		if bb_config.GlobalConfig.CliCfg.PXDesign != nil {
			binary = bb_config.GlobalConfig.CliCfg.PXDesign.Binary
		}
	}
	binary = strings.TrimSpace(binary)
	if binary == "" {
		return ""
	}
	current := filepath.Join(filepath.Dir(filepath.Dir(binary)), "current")
	info, err := os.Stat(current)
	if err != nil || !info.IsDir() {
		return ""
	}
	return current
}
