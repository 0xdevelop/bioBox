// Package ability_user_profile 注册用户资料自管理的对外方法（昵称修改等）。
// UserName 是注册必填主标识，不提供修改接口；NickName 创建时默认取 UserName，可改。
package bb_ability_user_profile

import (
	"context"
	"errors"

	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_user"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_common"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_session"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_error_code"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_supported_methods"
)

const MethodNicknameChange = "user.nickname.change"

func LoadAPIMethods() {
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{
		Name:        MethodNicknameChange,
		Description: "修改当前用户昵称",
		InputSchema: bb_api_auth_common.InputSchema(
			map[string]interface{}{
				"nick_name": bb_api_auth_common.StringSchema("", 1, 32),
			},
			"nick_name",
		),
		Execute: UserNicknameChange,
	})
}

func UserNicknameChange(ctx context.Context, input interface{}) (interface{}, error) {
	params, ok := bb_api_auth_common.InputObject(input)
	if !ok || !bb_api_auth_common.HasOnlyKeys(params, "nick_name") {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	nickName, nickOK := bb_api_auth_common.RequiredRawString(params, "nick_name")
	if !nickOK {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	user, _, err := bb_api_auth_session.AuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}
	updatedUser, err := bb_ability_user.NicknameChangeByUserID(ctx, user.UserID, nickName)
	if errors.Is(err, bb_ability_user.ErrInvalidNickName) {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"user": updatedUser}, nil
}
