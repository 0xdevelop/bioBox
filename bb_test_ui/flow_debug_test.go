package bb_test_ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/0xdevelop/bioBox/bb_ability"
)

func TestFlowDebugPageAndCatalogIncludesArtifactMethods(t *testing.T) {
	if len(flowDebugPage) == 0 {
		t.Fatal("flow_debug.html was not embedded")
	}
	bb_ability.LoadAbilityAPIMethods()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /flow_debug.html", handleFlowDebugPage)
	mux.HandleFunc("GET /api/config", handleConfig)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	pageResponse, err := http.Get(server.URL + "/flow_debug.html")
	if err != nil {
		t.Fatal(err)
	}
	defer pageResponse.Body.Close()
	page, err := io.ReadAll(pageResponse.Body)
	if err != nil {
		t.Fatal(err)
	}
	if pageResponse.StatusCode != http.StatusOK {
		t.Fatalf("flow_debug.html HTTP %d", pageResponse.StatusCode)
	}
	if !strings.Contains(pageResponse.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("unexpected content type %q", pageResponse.Header.Get("Content-Type"))
	}
	body := string(page)
	for _, needle := range []string{
		"只填服务地址和 API key",
		"task.artifacts",
		"task.artifact",
		"后端上报",
		"不用动画伪造进度",
		"/api/proxy",
		"PARSE_CIF_BEGIN",
		"结构展示只使用产物坐标，旋转不是折叠过程",
		"CA 连线只按残基序号，不是化学键验证",
		"没有结构就不用随机点冒充计算",
		"任务流协议",
		"function selectedTransport",
		"async function callSelected",
		"application/json, text/event-stream",
		"io.modelcontextprotocol/protocolVersion",
		"function mcpMeta",
		"mcp-task.list",
		"websocket-task.list",
		"grpc-task.list",
		`await callSelected("task.get"`,
		`await callSelected("task.artifacts"`,
		`await callSelected("task.artifact"`,
		`await callSelected("task.cancel"`,
		"await callSelected(method, args, true)",
		"pxdesign（短脚本 HTTP 已验，非正式 GPU 设计）",
		"流程轨道只点亮真实事件",
		"光纤上的光点表示这些事件的传递，不是推理计算进度",
		`id="flow-rail"`,
		"function drawFlowRail",
		"function markFlow",
		"悬停查看残基",
		"历史任务",
		"来自所选协议的",
		`id="history-rows"`,
		"function renderHistory",
		"async function loadHistory",
		`await callSelected("task.list"`,
		"该 API key 下还没有任务",
		"探测或刷新后列出；没有任务就空着",
		"function resetTaskView",
		"function stageAtoms",
		"stageTaskId",
		"不会继续显示上一任务的原子",
		"忽略过期产物",
		"alreadyReset",
		"上一任务产物已从舞台撤下",
		"只绑定当前 task_id",
		"含失败/取消/系统错误",
		"已按 task.artifacts 列出真实产物",
		"async function loadArtifacts(taskId, options)",
		`{ autoFetch: autoFetch }`,
		"{ alreadyReset: true }",
		"不会回退到 v1 binary",
		"仍缺授权权重",
		"cli_cfg.protenix.version_runtimes.v2",
		"Three.js / WebGL",
		"最终预测结构 / 不同样本，不是物理折叠过程",
		"structure-model",
		"structure-chain",
		"backboneSegments",
		"2.5–4.5 Å",
		"request !== state.artifactRequest",
		"prefers-reduced-motion",
		"仅在浏览器内读取，不上传",
		"state.three.dispose()",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("flow_debug.html missing %q", needle)
		}
	}
	for _, banned := range []string{
		"Math.random()",
		"fakeProgress",
		"const spread =",
		"atom.x + Math.cos(ang) * spread",
		"模拟进度百分比",
		`await callJSONRPC("task.get"`,
		`await callJSONRPC("task.artifacts"`,
		`await callJSONRPC("task.artifact"`,
		`await callJSONRPC("task.cancel"`,
		"await callJSONRPC(method, args, true)",
		"终态不是 success，未自动取件",
		"尚未完成独立 v1/v2 runtime 切换",
		"实际调用仍是单一配置 binary",
	} {
		if strings.Contains(body, banned) {
			t.Fatalf("flow_debug.html should not contain %q", banned)
		}
	}

	configResponse, err := http.Get(server.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	defer configResponse.Body.Close()
	if configResponse.StatusCode != http.StatusOK {
		t.Fatalf("api/config HTTP %d", configResponse.StatusCode)
	}
	var payload struct {
		Methods []struct {
			Name string `json:"name"`
		} `json:"methods"`
	}
	if err = json.NewDecoder(configResponse.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	present := map[string]bool{}
	for _, method := range payload.Methods {
		present[method.Name] = true
	}
	for _, name := range []string{"test", "protenix", "pxdesign", "task.get", "task.list", "task.cancel", "task.artifacts", "task.artifact"} {
		if !present[name] {
			t.Fatalf("test_ui catalog missing method %s", name)
		}
	}
}

func TestParseOfficialProtenixMMCIF(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("node is required to lock parseCIF against the official smoke CIF")
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test file path")
	}
	script := filepath.Join(filepath.Dir(thisFile), "testdata", "parse_official_cif.mjs")
	cmd := exec.Command(node, script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("parse official cif: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "parse_official_cif ok") {
		t.Fatalf("unexpected parser output: %s", out)
	}
}

func TestStructureSelectionAndStaleResponses(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("node is required for structure interaction regression")
	}
	_, thisFile, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(thisFile), "testdata", "structure_interactions.mjs")
	out, err := exec.Command(node, script).CombinedOutput()
	if err != nil {
		t.Fatalf("structure interaction regression: %v\n%s", err, out)
	}
}

func TestEmbeddedThreeStageIsBundledForRelease(t *testing.T) {
	webFS, err := embeddedTestUIStatic()
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	attachTestUIStatic(mux, webFS)
	server := httptest.NewServer(mux)
	defer server.Close()
	response, err := http.Get(server.URL + "/structure_stage.js")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(response.Header.Get("Content-Type"), "javascript") {
		t.Fatalf("stage bundle HTTP %d, type %s", response.StatusCode, response.Header.Get("Content-Type"))
	}
	for _, needle := range []string{"createStructureStage", "WebGL", "原始产物坐标"} {
		if !strings.Contains(string(body), needle) {
			t.Fatalf("embedded stage missing %s", needle)
		}
	}
	for _, needle := range []string{"https://cdn", "unpkg.com", `from"three"`, `from "three"`} {
		if strings.Contains(string(body), needle) {
			t.Fatalf("stage must bundle runtime imports, found %s", needle)
		}
	}
}
