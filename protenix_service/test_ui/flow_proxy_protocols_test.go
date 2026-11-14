package test_ui

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xdevelop/protenix_service/ability"
	"github.com/0xdevelop/protenix_service/ability/ability_user"
	"github.com/0xdevelop/protenix_service/api/api_auth/api_auth_config"
	"github.com/0xdevelop/protenix_service/api/api_config"
	"github.com/0xdevelop/protenix_service/api/api_grpc"
	"github.com/0xdevelop/protenix_service/api/api_grpc/api_config_grpc"
	"github.com/0xdevelop/protenix_service/api/api_jsonRPC"
	"github.com/0xdevelop/protenix_service/api/api_jsonRPC/api_config_jsonRPC"
	"github.com/0xdevelop/protenix_service/api/api_mcp"
	"github.com/0xdevelop/protenix_service/api/api_mcp/api_config_mcp"
	"github.com/0xdevelop/protenix_service/api/api_websocket"
	"github.com/0xdevelop/protenix_service/api/api_websocket/api_config_websocket"
	"github.com/0xdevelop/protenix_service/config"
	"github.com/0xdevelop/protenix_service/policy"
	"github.com/0xdevelop/protenix_service/policy/policy_config"
	"github.com/0xdevelop/protenix_service/storage"
	"github.com/0xdevelop/protenix_service/storage/storage_config"
)

// TestFlowDebugProxyRunsTaskFlowOverLiveProtocols starts the four real
// loopback API listeners plus test_ui /api/proxy, then exercises the same
// request shapes flow_debug.html uses when it is served same-origin.
// This is not a Linux browser click acceptance.
func TestFlowDebugProxyRunsTaskFlowOverLiveProtocols(t *testing.T) {
	root := t.TempDir()
	script := writeProxyFixtureScript(t, root)
	fixture := newLiveProtocolFixture(t, script, root)

	stopPolicy := policy.PolicyServicesStart()
	t.Cleanup(stopPolicy)
	input := writeProxyFixtureFile(t, root, "quick.json", "ok\n")
	accepted := fixture.call(t, "jsonrpc", "protenix", map[string]interface{}{
		"input": input, "out_dir": "out", "version": "v1",
	}, true, false)
	taskID, _ := accepted["task_id"].(string)
	if taskID == "" || accepted["run_status"] != "queued" {
		t.Fatal("short-script task was not durably accepted")
	}
	if task := fixture.waitDone(t, taskID, 10*time.Second); task["process_status"] != "success" {
		t.Fatalf("fixture task status=%v", task["process_status"])
	}

	for _, transport := range []string{"jsonrpc", "mcp", "websocket", "grpc"} {
		public := fixture.call(t, transport, "test", map[string]interface{}{}, false, false)
		if !publicHasSuccessText(public) {
			t.Fatalf("%s public test unexpected result keys=%v", transport, keysOf(public))
		}

		fixture.call(t, transport, "task.list", map[string]interface{}{}, false, true)
		fixture.call(t, transport, "task.list", map[string]interface{}{}, true, true, withCredential("ptxk_invalid"))

		listed := fixture.call(t, transport, "task.list", map[string]interface{}{}, true, false)
		if !taskListContains(listed, taskID) {
			t.Fatalf("%s task.list missing accepted task", transport)
		}

		got := fixture.call(t, transport, "task.get", map[string]interface{}{"task_id": taskID}, true, false)
		task, _ := got["task"].(map[string]interface{})
		if task["task_id"] != taskID || task["process_status"] != "success" {
			t.Fatalf("%s task.get mismatch status=%v", transport, task["process_status"])
		}

		artifacts := fixture.call(t, transport, "task.artifacts", map[string]interface{}{"task_id": taskID}, true, false)
		cifPath := cifArtifactPath(artifacts)
		if cifPath == "" {
			t.Fatalf("%s task.artifacts missing cif: keys=%v", transport, keysOf(artifacts))
		}
		file := fixture.call(t, transport, "task.artifact", map[string]interface{}{
			"task_id": taskID, "path": cifPath,
		}, true, false)
		decoded, err := base64.StdEncoding.DecodeString(fmt.Sprint(file["content"]))
		if err != nil || string(decoded) != "proxy_fixture_cif\n" || file["encoding"] != "base64" || file["kind"] != "cif" {
			t.Fatalf("%s task.artifact content mismatch kind=%v encoding=%v", transport, file["kind"], file["encoding"])
		}
	}
	t.Log("test_ui /api/proxy accepted public test, task.list/get/artifacts/artifact over live JSON-RPC, MCP, WebSocket, and gRPC; missing and forged keys were rejected; short-script CIF matched")
}

type liveProtocolFixture struct {
	proxy                              *httptest.Server
	key                                string
	jsonRPCURL, mcpURL, wsURL, grpcURL string
}

type callOption func(*callOptions)

type callOptions struct {
	credential string
	hasCred    bool
}

func withCredential(value string) callOption {
	return func(options *callOptions) {
		options.credential = value
		options.hasCred = true
	}
}

func newLiveProtocolFixture(t *testing.T, binary, root string) *liveProtocolFixture {
	t.Helper()
	previousConfig, previousAuth, previousAPI, previousPolicy := config.GlobalConfig, api_auth_config.CurrentCfgAuth, api_config.CurrentApiCfg, policy_config.CurrentCfgPolicy
	t.Cleanup(func() {
		api_jsonRPC.StopApiServiceWithJsonRPC()
		api_mcp.StopApiServiceWithMCP()
		api_websocket.StopApiServiceWithWebSocket()
		api_grpc.StopApiServiceWithGRPC()
		_ = storage.CloseStorage()
		config.GlobalConfig = previousConfig
		api_auth_config.CurrentCfgAuth = previousAuth
		api_config.CurrentApiCfg = previousAPI
		policy_config.CurrentCfgPolicy = previousPolicy
	})

	ports := reservedLoopbackPorts(t, 4)
	jsonRPCCfg := &api_config_jsonRPC.APIConfigJsonRPC{Enabled: true, BindAddress: "127.0.0.1", Port: ports[0]}
	mcpCfg := &api_config_mcp.APIConfigMCP{Enabled: true, BindAddress: "127.0.0.1", Port: ports[1], MCPTransportType: api_config_mcp.MCPTransportTypeStreamableHTTP}
	wsCfg := &api_config_websocket.APIConfigWebSocket{Enabled: true, BindAddress: "127.0.0.1", Port: ports[2], AllowedOrigins: []string{"127.0.0.1:*", "localhost:*"}}
	grpcCfg := &api_config_grpc.APIConfigGRPC{Enabled: true, BindAddress: "127.0.0.1", Port: ports[3]}

	config.GlobalConfig = &config.FileConfig{
		StorageCfg: storage_config.Default(filepath.Join(root, "queue.db")),
		CliCfg:     &config.CLIConfig{WorkRoot: filepath.Join(root, "jobs"), CancelGraceSeconds: 1, Protenix: &config.CLIBinaryConfig{Binary: binary, ExtraEnv: map[string]string{}}},
		ApiCfg:     &api_config.ApiConfig{APICfgJsonRPC: jsonRPCCfg, APICfgMCP: mcpCfg, APICfgWebSocket: wsCfg, APICfgGRPC: grpcCfg},
	}
	api_auth_config.CurrentCfgAuth = &api_auth_config.AuthConfig{Session: &api_auth_config.SessionConfig{
		JWTSigningSecret: "disposable-test-signing-secret-at-least-32-bytes", Issuer: "proxy-protocol-test", Audience: "proxy-protocol-test-client", AccessTokenTTLSeconds: 600, RefreshTokenTTLSeconds: 1200,
	}}
	api_config.CurrentApiCfg = config.GlobalConfig.ApiCfg
	policy_config.CurrentCfgPolicy = &policy_config.PolicyConfig{PolicyDuration: "30ms", WorkersScaller: 1, MaxWorkers: 2}
	if err := storage.OpenStorage(); err != nil {
		t.Fatal(err)
	}
	if err := storage.GormAutoMigrate(); err != nil {
		t.Fatal(err)
	}
	db, err := storage.GormDB(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ability_user.Create(db, "proxy_fixture", "proxy-fixture@example.invalid", "disposable-fixture-password", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	ability.LoadAbilityAPIMethods()

	api_jsonRPC.StartAPIServiceWithJsonRPC(jsonRPCCfg)
	api_mcp.StartAPIServiceWithMCP(mcpCfg)
	api_websocket.StartAPIServiceWithWebSocket(wsCfg)
	api_grpc.StartAPIServiceWithGRPC(grpcCfg)
	waitLoopback(t, fmt.Sprintf("127.0.0.1:%d", ports[0]))
	waitLoopback(t, fmt.Sprintf("127.0.0.1:%d", ports[1]))
	waitLoopback(t, fmt.Sprintf("127.0.0.1:%d", ports[2]))
	waitLoopback(t, fmt.Sprintf("127.0.0.1:%d", ports[3]))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/proxy", handleProxy)
	mux.HandleFunc("GET /flow_debug.html", handleFlowDebugPage)
	proxy := httptest.NewServer(mux)
	t.Cleanup(proxy.Close)

	fixture := &liveProtocolFixture{
		proxy:      proxy,
		jsonRPCURL: fmt.Sprintf("http://127.0.0.1:%d/", ports[0]),
		mcpURL:     fmt.Sprintf("http://127.0.0.1:%d/", ports[1]),
		wsURL:      fmt.Sprintf("ws://127.0.0.1:%d/", ports[2]),
		grpcURL:    fmt.Sprintf("grpc://127.0.0.1:%d", ports[3]),
	}

	page, err := http.Get(proxy.URL + "/flow_debug.html")
	if err != nil {
		t.Fatal(err)
	}
	defer page.Body.Close()
	if page.StatusCode != http.StatusOK {
		t.Fatalf("flow_debug.html HTTP %d", page.StatusCode)
	}

	login := fixture.callJSONRPCDirect(t, "auth.login.email", map[string]interface{}{
		"login_method": "password",
		"email":        "proxy-fixture@example.invalid",
		"password":     "disposable-fixture-password",
	}, "", false)
	issued := fixture.callJSONRPCDirect(t, "auth.api_key.issue", map[string]interface{}{"name": "proxy-acceptance"}, login["jwt_token"].(string), false)
	fixture.key = issued["api_key"].(string)
	if !strings.HasPrefix(fixture.key, "ptxk_") {
		t.Fatal("instance credential prefix changed")
	}
	return fixture
}

func (f *liveProtocolFixture) call(t *testing.T, transport, name string, args map[string]interface{}, useAuth, wantError bool, opts ...callOption) map[string]interface{} {
	t.Helper()
	options := callOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	headers := map[string]string{}
	id := "proxy-" + transport + "-" + name
	body := toolBody(name, args, id)
	url := f.jsonRPCURL
	method := http.MethodPost
	switch transport {
	case "jsonrpc":
		headers["Content-Type"] = "application/json"
		url = f.jsonRPCURL
	case "mcp":
		headers["Content-Type"] = "application/json"
		headers["Accept"] = "application/json, text/event-stream"
		headers["Mcp-Protocol-Version"] = "2026-07-28"
		headers["Mcp-Method"] = "tools/call"
		headers["Mcp-Name"] = name
		body = mcpToolBody(name, args, id)
		url = f.mcpURL
	case "websocket":
		url = f.wsURL
		method = http.MethodGet
	case "grpc":
		url = f.grpcURL
		body = grpcToolBody(name, args, "proxy-"+transport+"-"+name)
	default:
		t.Fatalf("unknown transport %s", transport)
	}
	if useAuth || options.hasCred {
		key := f.key
		if options.hasCred {
			key = options.credential
		}
		if transport == "grpc" {
			headers["authorization"] = "Bearer " + key
		} else if key != "" {
			headers["Authorization"] = "Bearer " + key
		}
	}
	payload, err := json.Marshal(proxyRequest{
		Transport: transport,
		URL:       url,
		Method:    method,
		Headers:   headers,
		Body:      body,
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := f.proxy.Client().Post(f.proxy.URL+"/api/proxy", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	var proxied proxyResponse
	if err = json.Unmarshal(raw, &proxied); err != nil {
		t.Fatalf("%s %s: proxy response is not JSON", transport, name)
	}
	if response.StatusCode != http.StatusOK || proxied.ProtocolError {
		if wantError && response.StatusCode == http.StatusOK && proxied.ProtocolError {
			return map[string]interface{}{"protocol_error": true}
		}
		// Do not log bodies: some methods can echo credentials in fixtures.
		t.Fatalf("%s %s: proxy HTTP=%d protocolError=%t", transport, name, response.StatusCode, proxied.ProtocolError)
	}
	if transport != "grpc" && proxied.Status != http.StatusOK && proxied.Status != http.StatusSwitchingProtocols {
		detail := ""
		if !useAuth && name == "test" {
			detail = truncateForTest(proxied.Body, 300)
		}
		t.Fatalf("%s %s: upstream HTTP %d %s", transport, name, proxied.Status, detail)
	}
	value, isError, err := parseToolPayload(transport, proxied.Body)
	if err != nil {
		t.Fatalf("%s %s: %v", transport, name, err)
	}
	if isError != wantError {
		t.Fatalf("%s %s: isError=%t expected=%t", transport, name, isError, wantError)
	}
	return value
}

func (f *liveProtocolFixture) callJSONRPCDirect(t *testing.T, name string, args map[string]interface{}, credential string, wantError bool) map[string]interface{} {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, f.jsonRPCURL, strings.NewReader(toolBody(name, args, "direct-"+name)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "proxy-protocol-acceptance/1.0")
	if credential != "" {
		req.Header.Set("Authorization", "Bearer "+credential)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	value, isError, err := parseToolPayload("jsonrpc", string(raw))
	if err != nil {
		t.Fatalf("direct %s: %v", name, err)
	}
	if resp.StatusCode != http.StatusOK || isError != wantError {
		t.Fatalf("direct %s: HTTP=%d isError=%t expected=%t", name, resp.StatusCode, isError, wantError)
	}
	return value
}

func (f *liveProtocolFixture) waitDone(t *testing.T, id string, timeout time.Duration) map[string]interface{} {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		got := f.call(t, "jsonrpc", "task.get", map[string]interface{}{"task_id": id}, true, false)
		task, _ := got["task"].(map[string]interface{})
		if task["run_status"] == "done" {
			return task
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatalf("task %s did not finish within %s", id, timeout)
	return nil
}

func toolBody(name string, args map[string]interface{}, id string) string {
	if args == nil {
		args = map[string]interface{}{}
	}
	body, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "tools/call",
		"params":  map[string]interface{}{"name": name, "arguments": args},
	})
	if err != nil {
		panic(err)
	}
	return string(body)
}

func mcpToolBody(name string, args map[string]interface{}, id string) string {
	if args == nil {
		args = map[string]interface{}{}
	}
	body, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": args,
			"_meta": map[string]interface{}{
				"io.modelcontextprotocol/protocolVersion": "2026-07-28",
				"io.modelcontextprotocol/clientInfo": map[string]interface{}{
					"name": "flow_debug", "version": "1.0.0",
				},
				"io.modelcontextprotocol/clientCapabilities": map[string]interface{}{
					"extensions": map[string]interface{}{},
				},
			},
		},
	})
	if err != nil {
		panic(err)
	}
	return string(body)
}

func grpcToolBody(name string, args map[string]interface{}, id string) string {
	if args == nil {
		args = map[string]interface{}{}
	}
	body, err := json.Marshal(map[string]interface{}{
		"requestId": id,
		"method":    "tools/call",
		"params":    map[string]interface{}{"name": name, "arguments": args},
	})
	if err != nil {
		panic(err)
	}
	return string(body)
}

func parseToolPayload(transport, raw string) (map[string]interface{}, bool, error) {
	var envelope map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return nil, false, fmt.Errorf("response is not JSON")
	}
	if transport == "grpc" {
		if result, ok := envelope["result"].(map[string]interface{}); ok {
			envelope = result
		}
	}
	if errObj, ok := envelope["error"]; ok && errObj != nil {
		return map[string]interface{}{"error": errObj}, true, nil
	}
	result, _ := envelope["result"].(map[string]interface{})
	if result == nil {
		result = envelope
	}
	if isError, _ := result["isError"].(bool); isError {
		return result, true, nil
	}
	if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
		item, _ := content[0].(map[string]interface{})
		text, _ := item["text"].(string)
		if text != "" {
			var value map[string]interface{}
			if err := json.Unmarshal([]byte(text), &value); err == nil {
				return value, false, nil
			}
			return map[string]interface{}{"text": text}, false, nil
		}
	}
	if nested, ok := result["result"].(map[string]interface{}); ok {
		return nested, false, nil
	}
	return result, false, nil
}

func publicHasSuccessText(value map[string]interface{}) bool {
	const want = "this is test method, request is success"
	if value["text"] == want {
		return true
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return false
	}
	return strings.Contains(string(encoded), want)
}

func taskListContains(listed map[string]interface{}, taskID string) bool {
	tasks, _ := listed["tasks"].([]interface{})
	for _, raw := range tasks {
		task, _ := raw.(map[string]interface{})
		if task["task_id"] == taskID {
			return true
		}
	}
	return false
}

func cifArtifactPath(listed map[string]interface{}) string {
	items, _ := listed["artifacts"].([]interface{})
	for _, raw := range items {
		item, _ := raw.(map[string]interface{})
		path, _ := item["path"].(string)
		kind, _ := item["kind"].(string)
		if kind == "cif" || strings.HasSuffix(strings.ToLower(path), ".cif") {
			return path
		}
	}
	return ""
}

func keysOf(value map[string]interface{}) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	return keys
}

func writeProxyFixtureScript(t *testing.T, root string) string {
	t.Helper()
	script := writeProxyFixtureFile(t, root, "proxy-cli.sh", `#!/bin/sh
input=""; out=""
while [ "$#" -gt 0 ]; do
 case "$1" in --input) input="$2"; shift 2;; --out_dir) out="$2"; shift 2;; *) shift;; esac
done
mkdir -p "$out"
printf 'proxy_fixture_cif\n' > "$out/pred.cif"
`)
	if err := os.Chmod(script, 0700); err != nil {
		t.Fatal(err)
	}
	return script
}

func writeProxyFixtureFile(t *testing.T, root, name, value string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func reservedLoopbackPorts(t *testing.T, count int) []int {
	t.Helper()
	listeners := make([]net.Listener, 0, count)
	ports := make([]int, 0, count)
	defer func() {
		for _, listener := range listeners {
			_ = listener.Close()
		}
	}()
	for i := 0; i < count; i++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		listeners = append(listeners, listener)
		ports = append(ports, listener.Addr().(*net.TCPAddr).Port)
	}
	return ports
}

func waitLoopback(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("service not listening on %s", addr)
}

func truncateForTest(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
