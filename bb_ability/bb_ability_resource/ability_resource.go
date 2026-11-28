package bb_ability_resource

import (
	"context"
	"errors"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_common"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_session"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_error_code"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_supported_methods"
	"github.com/0xdevelop/bioBox/bb_common/bb_app_path"
	"github.com/0xdevelop/bioBox/bb_common/bb_resource"
	"github.com/0xdevelop/bioBox/bb_config"
	"sort"
)

func controller() (bb_resource.Controller, error) {
	root, err := bb_app_path.ExecutableDir()
	return bb_resource.Controller{Root: root, Sets: bb_config.ConfiguredResourceSets()}, err
}
func Status(ctx context.Context, input interface{}) (interface{}, error) {
	if _, err := bb_api_auth_session.RequireAdmin(ctx); err != nil {
		return nil, err
	}
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
	if _, err := bb_api_auth_session.RequireAdmin(ctx); err != nil {
		return nil, err
	}
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
func Sets(ctx context.Context, input interface{}) (interface{}, error) {
	if _, err := bb_api_auth_session.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	params, ok := bb_api_auth_common.InputObject(input)
	if !ok || !bb_api_auth_common.HasNoUnknownKeys(params) {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	configured := bb_config.ConfiguredResourceSets()
	ids := make([]string, 0, len(configured))
	for id, cfg := range configured {
		if cfg != nil && cfg.Enabled {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return map[string]interface{}{"sets": ids}, nil
}

func LoadAPIMethods() {
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{Name: "resource.sets", Description: "列出当前实例已启用的资源配置集合 ID。仅 admin。", RequireAdmin: true, InputSchema: bb_api_auth_common.InputSchema(map[string]interface{}{}), Execute: Sets})
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{Name: "resource.status", Description: "查询配置资源集合及最后下载字节快照（不是推理进度）；不下载。任务生命周期仍以 task.get 为准。仅 admin。", RequireAdmin: true, InputSchema: bb_api_auth_common.InputSchema(map[string]interface{}{"set": bb_api_auth_common.StringSchema("", 1, 128)}, "set"), Execute: Status})
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{Name: "resource.prepare", Description: "明确确认 status 返回的指纹，仅准备所指配置集合；沿用持久异步任务，不启动推理。仅 admin。", Async: true, RequireAdmin: true, InputSchema: bb_api_auth_common.InputSchema(map[string]interface{}{"set": bb_api_auth_common.StringSchema("", 1, 128), "confirmation_token": bb_api_auth_common.StringSchema("", 64, 64)}, "set", "confirmation_token"), Execute: Prepare})
}
