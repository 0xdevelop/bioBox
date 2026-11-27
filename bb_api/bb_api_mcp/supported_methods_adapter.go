package bb_api_mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_session"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_executer"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_supported_methods"
	"github.com/0xdevelop/bioBox/bb_config"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	mcpMaxRequestBodySize = 4 << 20
)

func newMCPHTTPHandler() http.Handler {
	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server {
			return newMCPServer()
		},
		&mcp.StreamableHTTPOptions{
			Stateless:           true,
			JSONResponse:        true,
			MaxRequestBodyBytes: mcpMaxRequestBodySize,
		},
	)
	crossOriginProtection := http.NewCrossOriginProtection()
	// 不按 Mcp-Protocol-Version 请求头筛请求：按 MCP 规范，该头是握手协商完成
	// 之后的请求才携带，initialize 请求不带；在这里做写死版本号的等值判断会把
	// 握手本身挡在外面，标准客户端第一步就收到非 MCP 响应（官方 TS SDK 表现为
	// `Unexpected content type: null`）。版本协商是 SDK 的职责；非 MCP 的误访问
	// 由路由 GET 分支的 HomeHandler 接住；浏览器的 OPTIONS 预检在此单独处理。
	origins := frontendPageOrigins()
	for _, origin := range origins {
		if err := crossOriginProtection.AddTrustedOrigin(origin); err != nil {
			continue
		}
	}
	return allowFrontendPage(origins, crossOriginProtection.Handler(handler))
}

func frontendPageOrigins() []string {
	cfg := bb_config.GlobalConfig
	if cfg == nil || cfg.FrontendCfg == nil || !bb_config.FrontendEnabled() || cfg.FrontendCfg.Port < 1 || cfg.FrontendCfg.Port > 65535 {
		return nil
	}
	port := strconv.Itoa(cfg.FrontendCfg.Port)
	bind := strings.TrimSpace(cfg.FrontendCfg.BindAddress)
	hosts := []string{bind}
	switch bind {
	case "":
		return nil
	case "127.0.0.1", "localhost":
		hosts = []string{"127.0.0.1", "localhost"}
	case "0.0.0.0", "::":
		hosts = []string{"127.0.0.1", "localhost"}
		addresses, _ := net.InterfaceAddrs()
		for _, address := range addresses {
			network, ok := address.(*net.IPNet)
			if ok && network.IP.To4() != nil && !network.IP.IsLoopback() {
				hosts = append(hosts, network.IP.String())
			}
		}
	}
	origins := make([]string, 0, len(hosts))
	for _, host := range hosts {
		origins = append(origins, "http://"+net.JoinHostPort(host, port))
	}
	return origins
}

func allowFrontendPage(origins []string, next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, origin := range origins {
		allowed[origin] = true
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		writer.Header().Add("Vary", "Origin")
		if origin != "" && allowed[origin] {
			writer.Header().Set("Access-Control-Allow-Origin", origin)
			writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Mcp-Protocol-Version, Mcp-Method, Mcp-Name")
			writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			if request.Method == http.MethodOptions {
				writer.WriteHeader(http.StatusNoContent)
				return
			}
		}
		if request.Method == http.MethodOptions {
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func newMCPServer() *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    bb_config.ProjectName,
			Version: bb_config.ProjectVersion,
		},
		&mcp.ServerOptions{
			Capabilities: &mcp.ServerCapabilities{
				Tools: &mcp.ToolCapabilities{
					ListChanged: false,
				},
			},
			GetSessionID: func() string {
				return ""
			},
		},
	)
	server.AddReceivingMiddleware(explicitIsErrorMiddleware)

	for _, method := range bb_api_supported_methods.Methods() {
		method := method
		server.AddTool(
			&mcp.Tool{
				Name:        method.Name,
				Description: method.Description,
				InputSchema: method.InputSchemaWithoutGateToken(),
			},
			func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return executeSupportedMethod(ctx, request, method.Name)
			},
		)
	}
	return server
}

func executeSupportedMethod(ctx context.Context, request *mcp.CallToolRequest, methodName string) (*mcp.CallToolResult, error) {
	var arguments interface{} = map[string]interface{}{}
	if request != nil && request.Params != nil &&
		len(request.Params.Arguments) > 0 {
		if err := json.Unmarshal(request.Params.Arguments, &arguments); err != nil {
			return nil, &jsonrpc.Error{
				Code:    jsonrpc.CodeInvalidParams,
				Message: "tool arguments must be valid JSON",
			}
		}
	}

	userAgent := mcpRequestUserAgent(request)
	// 门禁凭证走标准 MCP HTTP Authorization: Bearer 头。
	ctx = bb_api_auth_session.WithBearerToken(ctx, mcpRequestHeader(request, "Authorization"))
	callParams := map[string]interface{}{
		"name":      methodName,
		"arguments": arguments,
	}
	encryptionKey := userAgent + "/"
	result, err := bb_api_executer.APIExecuter(
		ctx,
		bb_api_executer.ToolsCallMethod,
		callParams,
		encryptionKey,
	)
	if err != nil {
		if !errors.Is(err, bb_api_executer.ErrInvalidCall) {
			return nil, err
		}
		return nil, &jsonrpc.Error{
			Code:    jsonrpc.CodeInvalidParams,
			Message: err.Error(),
		}
	}
	return result.CallToolResult, nil
}

type explicitIsErrorResult struct {
	mcp.ResultBase
	result *mcp.CallToolResult
}

func (result *explicitIsErrorResult) MarshalJSON() ([]byte, error) {
	return bb_api_executer.MarshalCallToolResult(result.result, result.GetMeta())
}

func explicitIsErrorMiddleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
		result, err := next(ctx, method, request)
		if err != nil || method != bb_api_executer.ToolsCallMethod {
			return result, err
		}
		toolResult, ok := result.(*mcp.CallToolResult)
		if !ok {
			return result, nil
		}
		return &explicitIsErrorResult{
			ResultBase: mcp.ResultBase{Meta: toolResult.Meta},
			result:     toolResult,
		}, nil
	}
}

func mcpRequestUserAgent(request *mcp.CallToolRequest) string {
	return mcpRequestHeader(request, "User-Agent")
}

func mcpRequestHeader(request *mcp.CallToolRequest, name string) string {
	if request != nil && request.Extra != nil {
		return request.Extra.Header.Get(name)
	}
	return ""
}
