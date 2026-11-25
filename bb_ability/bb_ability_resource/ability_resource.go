package bb_ability_resource

import (
	"context"
	"errors"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_common"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_supported_methods"
	"github.com/0xdevelop/bioBox/bb_common/bb_app_path"
	"github.com/0xdevelop/bioBox/bb_common/bb_resource"
	"github.com/0xdevelop/bioBox/bb_config"
)

func controller() (bb_resource.Controller, error) {
	root, err := bb_app_path.ExecutableDir()
	return bb_resource.Controller{Root: root, Sets: bb_config.ConfiguredResourceSets()}, err
}
func Status(ctx context.Context, input interface{}) (interface{}, error) {
	p, ok := bb_api_auth_common.InputObject(input)
	if !ok {
		return nil, errors.New("resource arguments must be object")
	}
	id, ok := bb_api_auth_common.RequiredString(p, "set")
	if !ok {
		return nil, errors.New("resource set required")
	}
	c, err := controller()
	if err != nil {
		return nil, err
	}
	return c.Status(ctx, id)
}
func Prepare(ctx context.Context, input interface{}) (interface{}, error) {
	p, ok := bb_api_auth_common.InputObject(input)
	if !ok {
		return nil, errors.New("resource arguments must be object")
	}
	id, ok := bb_api_auth_common.RequiredString(p, "set")
	if !ok {
		return nil, errors.New("resource set required")
	}
	token, ok := bb_api_auth_common.RequiredString(p, "confirmation_token")
	if !ok || len(token) != 64 {
		return nil, bb_resource.ErrConfirmationRequired
	}
	c, err := controller()
	if err != nil {
		return nil, err
	}
	return c.Prepare(ctx, id, token)
}
func LoadAPIMethods() {
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{Name: "resource.status", Description: "查询配置资源集合及最后下载字节快照（不是推理进度）；不下载。任务生命周期仍以 task.get 为准。", InputSchema: bb_api_auth_common.InputSchema(map[string]interface{}{"set": bb_api_auth_common.StringSchema("", 1, 128)}, "set"), Execute: Status})
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{Name: "resource.prepare", Description: "明确确认 status 返回的指纹，仅准备所指配置集合；沿用持久异步任务，不启动推理。", Async: true, InputSchema: bb_api_auth_common.InputSchema(map[string]interface{}{"set": bb_api_auth_common.StringSchema("", 1, 128), "confirmation_token": bb_api_auth_common.StringSchema("", 64, 64)}, "set", "confirmation_token"), Execute: Prepare})
}
