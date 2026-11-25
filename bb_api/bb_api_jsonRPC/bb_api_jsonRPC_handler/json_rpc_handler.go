package bb_api_jsonRPC_handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_session"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_executer"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_jsonRPC/bb_api_jsonRPC_protocol"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_jsonRPC/bb_api_jsonRPC_request"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_jsonRPC/bb_api_jsonRPC_response"
)

// APIJsonRPCHandler 处理 JSON-RPC HTTP 请求。
func APIJsonRPCHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		bodyErr := errors.New("body read error")
		bb_api_jsonRPC_response.HandleResponse(w, bodyErr, nil, nil)
		return
	}

	request := &bb_api_jsonRPC_protocol.RPCRequest{}
	err = bb_api_jsonRPC_request.ParserRequest(body, request)
	if err != nil {
		bb_api_jsonRPC_response.HandleResponse(w, err, nil, request.ID)
		return
	}

	encryptionKey := fmt.Sprintf("%s/%s", r.UserAgent(), request.IDString())
	result, err := bb_api_executer.APIExecuter(
		bb_api_auth_session.WithBearerToken(r.Context(), r.Header.Get("Authorization")),
		request.Method,
		request.Params,
		encryptionKey,
	)
	if err != nil {
		if !errors.Is(err, bb_api_executer.ErrInvalidCall) {
			bb_api_jsonRPC_response.HandleServiceError(w, request.ID)
			return
		}
		bb_api_jsonRPC_response.HandleResponse(w, &bb_api_jsonRPC_protocol.RPCError{
			Code:    -32602,
			Message: err.Error(),
		}, nil, request.ID)
		return
	}
	if request.IsNotification() {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	bb_api_jsonRPC_response.HandleResponse(w, nil, &bb_api_jsonRPC_protocol.RPCResponse{
		Result: result,
	}, request.ID)
}
