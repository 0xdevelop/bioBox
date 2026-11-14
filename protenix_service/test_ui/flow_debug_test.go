package test_ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/0xdevelop/protenix_service/ability"
)

func TestFlowDebugPageAndCatalogIncludesArtifactMethods(t *testing.T) {
	if len(flowDebugPage) == 0 {
		t.Fatal("flow_debug.html was not embedded")
	}
	ability.LoadAbilityAPIMethods()
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
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("flow_debug.html missing %q", needle)
		}
	}
	for _, banned := range []string{"Math.random()", "fakeProgress", "模拟进度百分比"} {
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
