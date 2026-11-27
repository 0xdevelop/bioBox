// Package ability ability/ability.go
package bb_ability

import (
	"context"
	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_adapter_protenix"
	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_adapter_pxdesign"
	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_resource"

	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_protenix"
	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_pxdesign"
	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_task"
	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_user/bb_ability_user_profile"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_supported_methods"
)

const MethodTest = "test"

func Test(context.Context, interface{}) (interface{}, error) {
	return "this is test method, request is success", nil
}

func LoadAbilityAPIMethods() {
	bb_api_supported_methods.SupportedMethodsSetup()
	bb_api_supported_methods.AddMethod(
		&bb_api_supported_methods.SupportedMethod{
			Name:        MethodTest,
			Description: "检查统一 API 调用链是否可用",
			Public:      true,
			InputSchema: map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
			},
			Execute: Test,
		})
	bb_api_auth.LoadAPIMethods()
	// user_profile 调用父包 ability_user 的数据方法，父包带子包装配会成 import 环
	//（user → profile → user），故作为契约明记的唯一例外由顶层装配。
	bb_ability_user_profile.LoadAPIMethods()
	bb_ability_task.LoadAPIMethods()
	bb_ability_resource.LoadAPIMethods()
	bb_ability_adapter_protenix.Load()
	bb_ability_adapter_pxdesign.Load()
	bb_ability_protenix.LoadAPIMethods()
	bb_ability_pxdesign.LoadAPIMethods()
}
