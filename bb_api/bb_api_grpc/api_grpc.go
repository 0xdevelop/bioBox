package bb_api_grpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_session"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_config"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_executer"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_grpc/bb_api_config_grpc"
	bb_api_grpc_protobuf "github.com/0xdevelop/bioBox/bb_api/bb_api_grpc/bb_protobuf"
	"github.com/george012/gtbox/gtbox_log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	grpcMaxReceiveMessageSize = 4 << 20
	grpcMaxSendMessageSize    = 8 << 20
)

var grpcServer *grpc.Server

type apiServer struct {
	bb_api_grpc_protobuf.UnimplementedAPIServiceServer
}

func (server *apiServer) Call(ctx context.Context, request *bb_api_grpc_protobuf.CallRequest) (*bb_api_grpc_protobuf.CallResponse, error) {
	if request == nil || request.GetRequestId() == "" {
		return nil, status.Error(codes.InvalidArgument, "request_id is required")
	}

	var params interface{}
	if request.GetParams() != nil {
		params = request.GetParams().AsMap()
	}
	encryptionKey := fmt.Sprintf(
		"%s/%s",
		grpcUserAgent(ctx),
		request.GetRequestId(),
	)
	result, err := bb_api_executer.APIExecuter(
		bb_api_auth_session.WithBearerToken(ctx, grpcIncomingMetadata(ctx, "authorization")),
		request.GetMethod(),
		params,
		encryptionKey,
	)
	if err != nil {
		if errors.Is(err, bb_api_executer.ErrInvalidCall) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, "API execution failed")
	}
	resultStruct, err := callToolResultStruct(result)
	if err != nil {
		return nil, status.Error(codes.Internal, "API result encode failed")
	}
	return &bb_api_grpc_protobuf.CallResponse{
		RequestId: request.GetRequestId(),
		Result:    resultStruct,
	}, nil
}

func callToolResultStruct(result *bb_api_executer.CallToolResult) (*structpb.Struct, error) {
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	value := make(map[string]interface{})
	if err = json.Unmarshal(encoded, &value); err != nil {
		return nil, err
	}
	return structpb.NewStruct(value)
}

func grpcUserAgent(ctx context.Context) string {
	return grpcIncomingMetadata(ctx, "user-agent")
}

func grpcIncomingMetadata(ctx context.Context, key string) string {
	incomingMetadata, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := incomingMetadata.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func newGRPCServer() *grpc.Server {
	server := grpc.NewServer(
		grpc.MaxRecvMsgSize(grpcMaxReceiveMessageSize),
		grpc.MaxSendMsgSize(grpcMaxSendMessageSize),
	)
	bb_api_grpc_protobuf.RegisterAPIServiceServer(server, &apiServer{})
	return server
}

func StartAPIServiceWithGRPC(apiCfgGRPC *bb_api_config_grpc.APIConfigGRPC) {
	if apiCfgGRPC == nil {
		gtbox_log.LogErrorf("gRPC API config is nil")
		return
	}
	bb_api_config_grpc.CurrentAPICfgGRPC = apiCfgGRPC
	if apiCfgGRPC.Port < 1 || apiCfgGRPC.Port > 65535 {
		gtbox_log.LogErrorf("gRPC API port must be between 1 and 65535")
		return
	}
	if apiCfgGRPC.BindAddress == "" {
		gtbox_log.LogErrorf("api_cfg.api_cfg_grpc.bind_address is required")
		return
	}

	addr := net.JoinHostPort(apiCfgGRPC.BindAddress, strconv.Itoa(apiCfgGRPC.Port))
	listener, err := net.Listen("tcp4", addr)
	if err != nil {
		gtbox_log.LogErrorf("Failed to start gRPC server: %v", err)
		return
	}

	server := newGRPCServer()
	grpcServer = server

	addr_show := addr
	is_pub_serving := false

	if apiCfgGRPC.BindAddress == "0.0.0.0" {
		addr_show = net.JoinHostPort("127.0.0.1", strconv.Itoa(apiCfgGRPC.Port))
		is_pub_serving = true

	}
	go func() {
		gtbox_log.LogInfof("gRPC server Run On  [tcp://%s] Remote On[tcp://%s] Pub Serving [%v]", addr_show, net.JoinHostPort(bb_api_config.LatestActiveIPWithLocal, strconv.Itoa(apiCfgGRPC.Port)), is_pub_serving)
		if serveErr := server.Serve(listener); serveErr != nil &&
			!errors.Is(serveErr, grpc.ErrServerStopped) {
			gtbox_log.LogErrorf(
				"gRPC server stopped unexpectedly: %v",
				serveErr,
			)
		}
	}()
}

func StopApiServiceWithGRPC() {
	server := grpcServer
	if server == nil {
		gtbox_log.LogInfof("gRPC server is not running")
		return
	}

	gtbox_log.LogInfof("Shutting down gRPC server...")
	stopped := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		gtbox_log.LogInfof("gRPC server stopped successfully")
	case <-time.After(5 * time.Second):
		server.Stop()
		gtbox_log.LogErrorf("gRPC server forced to stop after timeout")
	}
	grpcServer = nil
}
