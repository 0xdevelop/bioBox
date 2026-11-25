// Package api api/api_services.go
package bb_api

import (
	"github.com/0xdevelop/bioBox/bb_ability"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_bridge"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_config"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_session"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_config"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_grpc"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_jsonRPC"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_mcp"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_websocket"
	"github.com/george012/gtbox/gtbox_log"
)

func StartAPIServices(apiCfg *bb_api_config.ApiConfig) {
	bb_ability.LoadAbilityAPIMethods()
	bb_api_config.CurrentApiCfg = apiCfg

	// bridge 形态启动期预加载 IdP 公钥（fail-fast）：失败明示告警，
	// 门禁保持 fail-closed 全拒，不静默降级。
	if authType, authTypeErr := bb_api_auth_config.CurrentAuthType(); authTypeErr == nil &&
		authType == bb_api_auth_config.AuthTypeBridge {
		if err := bb_api_auth_session.InitializeBridge(); err != nil {
			gtbox_log.LogErrorf("bridge 初始化失败(auth_cfg.bridge/公钥): %v — 全部受保护方法将被拒绝", err)
		} else if bb_api_auth_config.RevocationEnabled() && !bb_api_auth_bridge.RevocationConfigured() {
			gtbox_log.LogWarnf("bridge 本地吊销未装配(SetupRevocation 未调用): 登出后令牌在过期前仍可用")
		}
	}

	if bb_api_config.CurrentApiCfg.APICfgJsonRPC != nil {
		if bb_api_config.CurrentApiCfg.APICfgJsonRPC.Enabled == true {
			bb_api_jsonRPC.StartAPIServiceWithJsonRPC(apiCfg.APICfgJsonRPC)
		}

	} else {
		gtbox_log.LogErrorf("StartAPIServices API not setup")
	}

	if bb_api_config.CurrentApiCfg.APICfgMCP != nil {
		if bb_api_config.CurrentApiCfg.APICfgMCP.Enabled == true {
			bb_api_mcp.StartAPIServiceWithMCP(apiCfg.APICfgMCP)
		}

	} else {
		gtbox_log.LogErrorf("StartAPIServices API not setup")
	}

	if bb_api_config.CurrentApiCfg.APICfgWebSocket != nil &&
		bb_api_config.CurrentApiCfg.APICfgWebSocket.Enabled {
		bb_api_websocket.StartAPIServiceWithWebSocket(
			apiCfg.APICfgWebSocket,
		)
	}

	if bb_api_config.CurrentApiCfg.APICfgGRPC != nil &&
		bb_api_config.CurrentApiCfg.APICfgGRPC.Enabled {
		bb_api_grpc.StartAPIServiceWithGRPC(apiCfg.APICfgGRPC)
	}
}

func StopApiServices() {
	bb_api_grpc.StopApiServiceWithGRPC()
	bb_api_websocket.StopApiServiceWithWebSocket()
	bb_api_mcp.StopApiServiceWithMCP()
	bb_api_jsonRPC.StopApiServiceWithJsonRPC()
}
