package bb_test_ui

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestTestUIPortConflict(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	listener, err := listen("127.0.0.1", occupied.Addr().(*net.TCPAddr).Port)
	if err == nil {
		defer listener.Close()
		t.Fatalf("occupied configured port must fail, listener moved to %s", listener.Addr())
	}
}

func TestHandleAPIDocPageEmbedFallbackAndConfigPaths(t *testing.T) {
	webFS, err := embeddedTestUIStatic()
	if err != nil {
		t.Fatalf("embeddedTestUIStatic: %v", err)
	}
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/docs_api", nil)
	handleAPIDocPage(rec, req, webFS)
	if rec.Code != http.StatusOK {
		t.Fatalf("/docs_api embed fallback HTTP %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "protenix") || !strings.Contains(body, "pxdesign") {
		t.Fatalf("/docs_api embed fallback missing expected API methods")
	}

	cfgRec := httptest.NewRecorder()
	cfgReq := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	handleConfig(cfgRec, cfgReq)
	if cfgRec.Code != http.StatusOK {
		t.Fatalf("/api/config HTTP %d", cfgRec.Code)
	}
	var payload webConfig
	if err := json.NewDecoder(cfgRec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Project.Name != "bioBox" {
		t.Fatalf("project.name=%q want bioBox", payload.Project.Name)
	}
	if payload.GRPCService != "protenix_service.api.v1.APIService" {
		t.Fatalf("grpcService=%q want protenix_service.api.v1.APIService", payload.GRPCService)
	}
	if payload.GRPCProtoImportPath != "bb_api/bb_api_grpc/proto" {
		t.Fatalf("grpcProtoImportPath=%q want bb_api/bb_api_grpc/proto", payload.GRPCProtoImportPath)
	}
}
