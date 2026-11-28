package bb_ability_admin

import (
	"context"
	"errors"
	"time"

	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_task"
	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_user"
	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_user/bb_ability_user_model"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_common"
	authModel "github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_model"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_session"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_error_code"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_supported_methods"
	"github.com/0xdevelop/bioBox/bb_storage"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	adminUserListDefault    = 100
	adminUserListMax        = 200
	adminSessionListDefault = 100
	adminSessionListMax     = 200
)

func LoadAPIMethods() {
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{
		Name:         "admin.user.list",
		Description:  "列出全部用户（不含密码哈希等秘密）。仅 admin。默认 100 条，最大 200 条，响应含 total。",
		RequireAdmin: true,
		InputSchema: bb_api_auth_common.InputSchema(map[string]interface{}{
			"limit":  bb_api_auth_common.IntegerSchema(1, adminUserListMax),
			"offset": bb_api_auth_common.IntegerSchema(0, 1_000_000_000),
		}),
		Execute: AdminUserList,
	})
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{
		Name:         "admin.user.set_role",
		Description:  "设置指定用户的 role（user 或 admin）。禁止撤销最后一个 admin。仅 admin。",
		RequireAdmin: true,
		InputSchema: bb_api_auth_common.InputSchema(map[string]interface{}{
			"user_id": bb_api_auth_common.StringSchema("", 36, 36),
			"role":    bb_api_auth_common.EnumStringSchema(bb_ability_user.RoleUser, bb_ability_user.RoleAdmin),
		}, "user_id", "role"),
		Execute: AdminUserSetRole,
	})
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{
		Name:         "admin.task.list",
		Description:  "列出全部用户的异步任务（新→旧）。仅 admin。默认 100 条，最大 200 条，响应含 total；可按 user_id 过滤。不要与 task.list 的 50 条上限混淆。",
		RequireAdmin: true,
		InputSchema: bb_api_auth_common.InputSchema(map[string]interface{}{
			"limit":   bb_api_auth_common.IntegerSchema(1, 200),
			"offset":  bb_api_auth_common.IntegerSchema(0, 1_000_000_000),
			"user_id": bb_api_auth_common.StringSchema("", 36, 36),
		}),
		Execute: bb_ability_task.AdminTaskList,
	})
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{
		Name:         "admin.task.get",
		Description:  "查看任意用户的异步任务状态、步进与结果。仅 admin。",
		RequireAdmin: true,
		InputSchema: bb_api_auth_common.InputSchema(map[string]interface{}{
			"task_id": bb_api_auth_common.StringSchema("", 36, 36),
		}, "task_id"),
		Execute: bb_ability_task.AdminTaskGet,
	})
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{
		Name:         "admin.task.cancel",
		Description:  "取消任意用户的排队或执行中任务。仅 admin。",
		RequireAdmin: true,
		InputSchema: bb_api_auth_common.InputSchema(map[string]interface{}{
			"task_id": bb_api_auth_common.StringSchema("", 36, 36),
		}, "task_id"),
		Execute: bb_ability_task.AdminTaskCancel,
	})
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{
		Name:         "admin.task.artifacts",
		Description:  "列出任意用户已完成任务的产物（相对路径）。仅 admin。",
		RequireAdmin: true,
		InputSchema: bb_api_auth_common.InputSchema(map[string]interface{}{
			"task_id": bb_api_auth_common.StringSchema("", 36, 36),
		}, "task_id"),
		Execute: bb_ability_task.AdminTaskArtifacts,
	})
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{
		Name:         "admin.task.artifact",
		Description:  "读取任意用户已完成任务的单个产物内容（base64）。仅 admin。",
		RequireAdmin: true,
		InputSchema: bb_api_auth_common.InputSchema(map[string]interface{}{
			"task_id": bb_api_auth_common.StringSchema("", 36, 36),
			"path":    bb_api_auth_common.StringSchema("", 1, 1024),
		}, "task_id", "path"),
		Execute: bb_ability_task.AdminTaskArtifact,
	})
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{
		Name:         "admin.session.list",
		Description:  "列出登录会话（不含 refresh token 哈希）。仅 admin。默认 100 条，最大 200 条，响应含 total；可按 user_id 过滤。",
		RequireAdmin: true,
		InputSchema: bb_api_auth_common.InputSchema(map[string]interface{}{
			"limit":   bb_api_auth_common.IntegerSchema(1, adminSessionListMax),
			"offset":  bb_api_auth_common.IntegerSchema(0, 1_000_000_000),
			"user_id": bb_api_auth_common.StringSchema("", 36, 36),
		}),
		Execute: AdminSessionList,
	})
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{
		Name:         "admin.session.revoke",
		Description:  "吊销指定登录会话。仅 admin。",
		RequireAdmin: true,
		InputSchema: bb_api_auth_common.InputSchema(map[string]interface{}{
			"session_id": bb_api_auth_common.StringSchema("", 36, 36),
		}, "session_id"),
		Execute: AdminSessionRevoke,
	})
}

func AdminUserList(ctx context.Context, input interface{}) (interface{}, error) {
	if _, err := bb_api_auth_session.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	params, ok := bb_api_auth_common.InputObject(input)
	if !ok || !bb_api_auth_common.HasNoUnknownKeys(params, "limit", "offset") {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	limit, limitOK := bb_api_auth_common.OptionalInt(params, "limit", adminUserListDefault, 1, adminUserListMax)
	offset, offsetOK := bb_api_auth_common.OptionalInt(params, "offset", 0, 0, 1_000_000_000)
	if !limitOK || !offsetOK {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}
	var total int64
	if err = db.Model(&bb_ability_user_model.User{}).Count(&total).Error; err != nil {
		return nil, err
	}
	users := make([]*bb_ability_user_model.User, 0)
	if err = db.Order("id DESC").Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		return nil, err
	}
	items := make([]map[string]interface{}, 0, len(users))
	for _, user := range users {
		items = append(items, bb_ability_user.PublicUser(user))
	}
	return map[string]interface{}{"users": items, "total": total, "limit": limit, "offset": offset}, nil
}

func AdminUserSetRole(ctx context.Context, input interface{}) (interface{}, error) {
	if _, err := bb_api_auth_session.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	params, ok := bb_api_auth_common.InputObject(input)
	if !ok || !bb_api_auth_common.HasOnlyKeys(params, "user_id", "role") {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	userID, userIDOK := bb_api_auth_common.RequiredString(params, "user_id")
	roleValue, roleOK := bb_api_auth_common.RequiredString(params, "role")
	if !userIDOK || !roleOK || uuid.Validate(userID) != nil {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	role, err := bb_ability_user.NormalizeRole(roleValue)
	if err != nil {
		return nil, err
	}
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}
	var updated *bb_ability_user_model.User
	err = db.Transaction(func(tx *gorm.DB) error {
		var txErr error
		updated, txErr = bb_ability_user.AssignRoleInTx(tx, userID, role)
		return txErr
	})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"user": bb_ability_user.PublicUser(updated)}, nil
}

func publicSession(session *authModel.AuthSession) map[string]interface{} {
	item := map[string]interface{}{
		"session_id": session.SessionID,
		"user_id":    session.UserID,
		"expires_at": session.ExpiresAt.UTC().Format(time.RFC3339),
		"created_at": session.CreatedAt.UTC().Format(time.RFC3339),
	}
	if session.RevokedAt != nil {
		item["revoked_at"] = session.RevokedAt.UTC().Format(time.RFC3339)
	} else {
		item["revoked_at"] = nil
	}
	return item
}

func AdminSessionList(ctx context.Context, input interface{}) (interface{}, error) {
	if _, err := bb_api_auth_session.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	params, ok := bb_api_auth_common.InputObject(input)
	if !ok || !bb_api_auth_common.HasNoUnknownKeys(params, "limit", "offset", "user_id") {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	limit, limitOK := bb_api_auth_common.OptionalInt(params, "limit", adminSessionListDefault, 1, adminSessionListMax)
	offset, offsetOK := bb_api_auth_common.OptionalInt(params, "offset", 0, 0, 1_000_000_000)
	if !limitOK || !offsetOK {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	var ownerID string
	if _, exists := params["user_id"]; exists {
		value, ok := bb_api_auth_common.RequiredString(params, "user_id")
		if !ok || uuid.Validate(value) != nil {
			return nil, bb_api_error_code.ErrInvalidArguments
		}
		ownerID = value
	}
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}
	query := db.Model(&authModel.AuthSession{})
	if ownerID != "" {
		query = query.Where("user_id = ?", ownerID)
	}
	var total int64
	if err = query.Count(&total).Error; err != nil {
		return nil, err
	}
	sessions := make([]*authModel.AuthSession, 0)
	if err = query.Order("id DESC").Limit(limit).Offset(offset).Find(&sessions).Error; err != nil {
		return nil, err
	}
	items := make([]map[string]interface{}, 0, len(sessions))
	for _, session := range sessions {
		items = append(items, publicSession(session))
	}
	return map[string]interface{}{"sessions": items, "total": total, "limit": limit, "offset": offset}, nil
}

func AdminSessionRevoke(ctx context.Context, input interface{}) (interface{}, error) {
	if _, err := bb_api_auth_session.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	params, ok := bb_api_auth_common.InputObject(input)
	if !ok || !bb_api_auth_common.HasOnlyKeys(params, "session_id") {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	sessionID, ok := bb_api_auth_common.RequiredString(params, "session_id")
	if !ok || uuid.Validate(sessionID) != nil {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}
	session := &authModel.AuthSession{}
	if err = db.Where("session_id = ?", sessionID).Take(session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, bb_api_error_code.ErrInvalidArguments
		}
		return nil, err
	}
	if session.RevokedAt == nil {
		now := time.Now().UTC()
		if err = db.Model(session).Update("revoked_at", now).Error; err != nil {
			return nil, err
		}
		session.RevokedAt = &now
	}
	return map[string]interface{}{"session": publicSession(session)}, nil
}
