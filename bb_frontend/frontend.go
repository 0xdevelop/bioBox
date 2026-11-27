package bb_frontend

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/0xdevelop/bioBox/bb_api/bb_api_config"
	"github.com/0xdevelop/bioBox/bb_frontend/bb_frontend_config"
	"github.com/george012/gtbox/gtbox_log"
)

//go:embed all:dist
var frontendFiles embed.FS

// FrontendServicesStart 按配置监听前端静态页。关闭或不具备监听地址时不监听，embed 仍留在二进制内。
func FrontendServicesStart(cfg *bb_frontend_config.FrontendConfig) (*http.Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg != nil && cfg.Enabled != nil && !*cfg.Enabled {
		gtbox_log.LogInfof("Frontend disabled: frontend_cfg.enabled=false")
		return nil, nil
	}
	if cfg == nil || strings.TrimSpace(cfg.BindAddress) == "" || cfg.Port < 1 || cfg.Port > 65535 {
		gtbox_log.LogInfof("Frontend disabled: frontend_cfg is not configured")
		return nil, nil
	}
	webFS, err := fs.Sub(frontendFiles, "dist")
	if err != nil {
		return nil, fmt.Errorf("load frontend assets: %w", err)
	}
	handler, err := frontendHandler(webFS)
	if err != nil {
		return nil, err
	}
	addr := net.JoinHostPort(cfg.BindAddress, strconv.Itoa(cfg.Port))
	listener, err := net.Listen("tcp4", addr)
	if err != nil {
		return nil, fmt.Errorf("listen frontend: %w", err)
	}
	server := &http.Server{
		Addr:              listener.Addr().String(),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	addr_show := addr
	is_pub_serving := false

	if cfg.BindAddress == "0.0.0.0" {
		addr_show = net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.Port))
		is_pub_serving = true

	}
	go func() {
		gtbox_log.LogInfof("Frontend Run On  [http://%s] Remote On[http://%s] Pub Serving [%v]", addr_show, net.JoinHostPort(bb_api_config.LatestActiveIPWithLocal, strconv.Itoa(cfg.Port)), is_pub_serving)
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			gtbox_log.LogErrorf("frontend stopped unexpectedly: %v", serveErr)
		}
	}()
	return server, nil
}

func frontendHandler(webFS fs.FS) (http.Handler, error) {
	index, err := fs.ReadFile(webFS, "index.html")
	if err != nil {
		return nil, fmt.Errorf("read frontend index: %w", err)
	}
	mcpPort := ""
	if cfg := bb_api_config.CurrentApiCfg; cfg != nil && cfg.APICfgMCP != nil && cfg.APICfgMCP.Enabled {
		mcpPort = strconv.Itoa(cfg.APICfgMCP.Port)
	}
	index = bytes.Replace(index, []byte("{{BIOBOX_MCP_PORT}}"), []byte(mcpPort), 1)
	static := frontendStaticHandler(webFS)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/" && (request.Method == http.MethodGet || request.Method == http.MethodHead) {
			writer.Header().Set("Cache-Control", "no-cache")
			http.ServeContent(writer, request, "index.html", time.Time{}, bytes.NewReader(index))
			return
		}
		if request.Method == http.MethodPost && path.Clean(request.URL.Path) == "/cli-input" {
			receiveCLIInput(writer, request)
			return
		}
		static.ServeHTTP(writer, request)
	}), nil
}

func frontendStaticHandler(webFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(webFS))
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			return
		}
		name := strings.TrimPrefix(path.Clean(request.URL.Path), "/")
		if name == "" || name == "." {
			name = "index.html"
		}
		info, err := fs.Stat(webFS, name)
		if err != nil || !info.Mode().IsRegular() {
			return
		}
		fileServer.ServeHTTP(writer, request)
	})
}
