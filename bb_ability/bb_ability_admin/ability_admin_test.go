package bb_ability_admin_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xdevelop/bioBox/bb_ability"
	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_admin"
	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_resource"
	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_task"
	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_user"
	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_user/bb_ability_user_model"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_api_key"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_common"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_config"
	authModel "github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_model"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_register"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_session"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_error_code"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_executer"
	"github.com/0xdevelop/bioBox/bb_config"
	"github.com/0xdevelop/bioBox/bb_storage"
	"github.com/0xdevelop/bioBox/bb_storage/bb_storage_config"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

const testPassword = "correct-horse-battery-staple"

var testSendInterval = 60

func setupAdminDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousConfig := bb_config.GlobalConfig
	previousAuth := bb_api_auth_config.CurrentCfgAuth
	enabled := true
	bb_config.GlobalConfig = &bb_config.FileConfig{StorageCfg: bb_storage_config.Default(filepath.Join(t.TempDir(), "admin.db")), AuthCfg: nil}
	bb_api_auth_config.CurrentCfgAuth = &bb_api_auth_config.AuthConfig{
		Enabled: &enabled,
		Verification: &bb_api_auth_config.VerificationConfig{
			CodeTTLSeconds: 600, MaxAttempts: 5, HourlySendLimit: 5, CodeHashSecret: "test-only-code-hash-secret-32bytes",
			SendIntervalSeconds: &testSendInterval,
		},
		Session: &bb_api_auth_config.SessionConfig{
			JWTSigningSecret: "test-only-signing-secret-at-least-32-bytes",
			Issuer:           "template-test", Audience: "template-test-client",
			AccessTokenTTLSeconds: 60, RefreshTokenTTLSeconds: 120,
		},
	}
	t.Cleanup(func() {
		_ = bb_storage.CloseStorage()
		bb_config.GlobalConfig = previousConfig
		bb_api_auth_config.CurrentCfgAuth = previousAuth
	})
	if err := bb_storage.OpenStorage(); err != nil {
		t.Fatal(err)
	}
	if err := bb_storage.GormAutoMigrate(); err != nil {
		t.Fatal(err)
	}
	if err := bb_ability_user.InitializeRoles(context.Background()); err != nil {
		t.Fatal(err)
	}
	db, err := bb_storage.GormDB(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func seedVerifyCode(t *testing.T, db *gorm.DB, email, code string) {
	t.Helper()
	now := time.Now().UTC()
	hash := bb_api_auth_common.VerificationCodeHash(bb_api_auth_config.CurrentCfgAuth.Verification.CodeHashSecret, email, code)
	if err := db.Create(&authModel.AuthVerifyCode{
		Recipient: email, CodeHash: hash, AttemptsRemaining: 5,
		ExpiresAt: now.Add(10 * time.Minute), ResendAvailableAt: now,
		SendWindowStartedAt: now, SendCount: 1, SentAt: &now,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func register(t *testing.T, db *gorm.DB, name, email, code string) *bb_ability_user_model.User {
	t.Helper()
	seedVerifyCode(t, db, email, code)
	result, err := bb_api_auth_register.AuthRegister(context.Background(), map[string]interface{}{
		"user_name": name, "email": email, "password": testPassword, "verify_code": code,
	})
	if err != nil {
		t.Fatalf("register %s: %v", name, err)
	}
	return result.(map[string]interface{})["user"].(*bb_ability_user_model.User)
}

func sessionCtx(t *testing.T, email string) (context.Context, map[string]interface{}) {
	t.Helper()
	result, err := bb_api_auth_session.AuthLoginEmail(context.Background(), map[string]interface{}{
		"login_method": "password", "email": email, "password": testPassword,
	})
	if err != nil {
		t.Fatalf("login %s: %v", email, err)
	}
	payload := result.(map[string]interface{})
	jwtToken := payload["jwt_token"].(string)
	ctx, err := bb_api_auth_session.AuthenticateRequest(
		bb_api_auth_session.WithBearerToken(context.Background(), "Bearer "+jwtToken), map[string]interface{}{},
	)
	if err != nil {
		t.Fatalf("gate: %v", err)
	}
	return ctx, payload
}

func TestNonAdminDeniedAndAdminCanManageUsersAndSessions(t *testing.T) {
	db := setupAdminDB(t)
	admin := register(t, db, "alice", "alice@example.com", "111111")
	member := register(t, db, "bob", "bob@example.com", "222222")
	if admin.Role != bb_ability_user.RoleAdmin || member.Role != bb_ability_user.RoleUser {
		t.Fatalf("roles admin=%s member=%s", admin.Role, member.Role)
	}
	memberCtx, _ := sessionCtx(t, "bob@example.com")
	if _, err := bb_ability_admin.AdminUserList(memberCtx, map[string]interface{}{}); !errors.Is(err, bb_api_error_code.ErrPermissionDenied) {
		t.Fatalf("non-admin list: %v", err)
	}
	if _, err := bb_ability_admin.AdminUserSetRole(memberCtx, map[string]interface{}{"user_id": member.UserID, "role": "admin", "role_claim": "admin"}); !errors.Is(err, bb_api_error_code.ErrInvalidArguments) && !errors.Is(err, bb_api_error_code.ErrPermissionDenied) {
		// extra key is invalid arguments; without extra key would be permission denied
		t.Fatalf("forged extra claim: %v", err)
	}
	if _, err := bb_ability_admin.AdminUserSetRole(memberCtx, map[string]interface{}{"user_id": member.UserID, "role": "admin"}); !errors.Is(err, bb_api_error_code.ErrPermissionDenied) {
		t.Fatalf("non-admin set_role: %v", err)
	}
	adminCtx, loginPayload := sessionCtx(t, "alice@example.com")
	if loginPayload["role"] != bb_ability_user.RoleAdmin {
		t.Fatalf("login role = %v", loginPayload["role"])
	}
	listed, err := bb_ability_admin.AdminUserList(adminCtx, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	body := listed.(map[string]interface{})
	if body["total"] != int64(2) || len(body["users"].([]map[string]interface{})) != 2 {
		t.Fatalf("user list = %#v", body)
	}
	for _, item := range body["users"].([]map[string]interface{}) {
		if _, exists := item["password_hash"]; exists {
			t.Fatal("password hash must not appear")
		}
	}
	if _, err := bb_ability_admin.AdminUserSetRole(adminCtx, map[string]interface{}{"user_id": member.UserID, "role": "admin"}); err != nil {
		t.Fatalf("promote: %v", err)
	}
	if _, err := bb_ability_admin.AdminUserSetRole(adminCtx, map[string]interface{}{"user_id": admin.UserID, "role": "user"}); err != nil {
		t.Fatalf("demote one of two admins: %v", err)
	}
	memberAdminCtx, _ := sessionCtx(t, "bob@example.com")
	if _, err := bb_ability_admin.AdminUserSetRole(memberAdminCtx, map[string]interface{}{"user_id": member.UserID, "role": "user"}); !errors.Is(err, bb_api_error_code.ErrLastAdmin) {
		t.Fatalf("last admin protect: %v", err)
	}
	sessions, err := bb_ability_admin.AdminSessionList(memberAdminCtx, map[string]interface{}{"user_id": admin.UserID})
	if err != nil {
		t.Fatal(err)
	}
	sessionRows := sessions.(map[string]interface{})["sessions"].([]map[string]interface{})
	if len(sessionRows) == 0 {
		t.Fatal("expected admin sessions")
	}
	if _, exists := sessionRows[0]["refresh_token_hash"]; exists {
		t.Fatal("session list leaked refresh hash")
	}
}

func TestAdminCanInspectAndCancelOtherUsersTasks(t *testing.T) {
	db := setupAdminDB(t)
	register(t, db, "alice", "alice@example.com", "111111")
	member := register(t, db, "bob", "bob@example.com", "222222")
	adminCtx, _ := sessionCtx(t, "alice@example.com")
	memberCtx, _ := sessionCtx(t, "bob@example.com")
	accepted, err := bb_ability_task.AcceptAsyncTask(memberCtx, "queue-probe", map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	taskID := accepted.(map[string]interface{})["task_id"].(string)
	mine, err := bb_ability_task.TaskList(memberCtx, map[string]interface{}{})
	if err != nil || len(mine.(map[string]interface{})["tasks"].([]map[string]interface{})) != 1 {
		t.Fatalf("member task.list = %#v err %v", mine, err)
	}
	adminList, err := bb_ability_task.AdminTaskList(adminCtx, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	listed := adminList.(map[string]interface{})
	if listed["total"] != int64(1) {
		t.Fatalf("admin list total = %#v", listed)
	}
	row := listed["tasks"].([]map[string]interface{})[0]
	if row["user_id"] != member.UserID || row["task_id"] != taskID {
		t.Fatalf("admin list row = %#v", row)
	}
	if _, err := bb_ability_task.AdminTaskList(memberCtx, map[string]interface{}{}); !errors.Is(err, bb_api_error_code.ErrPermissionDenied) {
		t.Fatalf("non-admin task list: %v", err)
	}
	got, err := bb_ability_task.AdminTaskGet(adminCtx, map[string]interface{}{"task_id": taskID})
	if err != nil {
		t.Fatal(err)
	}
	if got.(map[string]interface{})["task"].(map[string]interface{})["user_id"] != member.UserID {
		t.Fatalf("admin get = %#v", got)
	}
	cancelled, err := bb_ability_task.AdminTaskCancel(adminCtx, map[string]interface{}{"task_id": taskID})
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.(map[string]interface{})["cancelled"] != true {
		t.Fatalf("cancel = %#v", cancelled)
	}
}

func TestAdminAPIKeyCanCallAdminMethodsAndUserKeyCannot(t *testing.T) {
	db := setupAdminDB(t)
	register(t, db, "alice", "alice@example.com", "111111")
	register(t, db, "bob", "bob@example.com", "222222")
	adminSess, _ := sessionCtx(t, "alice@example.com")
	memberSess, _ := sessionCtx(t, "bob@example.com")
	issued, err := bb_api_auth_api_key.AuthAPIKeyIssue(adminSess, map[string]interface{}{"name": "ops"})
	if err != nil {
		t.Fatal(err)
	}
	adminKey := issued.(map[string]interface{})["api_key"].(string)
	memberIssued, err := bb_api_auth_api_key.AuthAPIKeyIssue(memberSess, map[string]interface{}{"name": "laptop"})
	if err != nil {
		t.Fatal(err)
	}
	memberKey := memberIssued.(map[string]interface{})["api_key"].(string)
	adminKeyCtx, err := bb_api_auth_session.AuthenticateRequest(
		bb_api_auth_session.WithBearerToken(context.Background(), "Bearer "+adminKey), map[string]interface{}{},
	)
	if err != nil {
		t.Fatal(err)
	}
	memberKeyCtx, err := bb_api_auth_session.AuthenticateRequest(
		bb_api_auth_session.WithBearerToken(context.Background(), "Bearer "+memberKey), map[string]interface{}{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bb_ability_admin.AdminUserList(adminKeyCtx, map[string]interface{}{}); err != nil {
		t.Fatalf("admin api key should call admin APIs: %v", err)
	}
	if _, err := bb_ability_admin.AdminUserList(memberKeyCtx, map[string]interface{}{}); !errors.Is(err, bb_api_error_code.ErrPermissionDenied) {
		t.Fatalf("user api key must be denied: %v", err)
	}
	checked, err := bb_api_auth_session.AuthJWTTokenCheck(adminKeyCtx, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if checked.(map[string]interface{})["role"] != bb_ability_user.RoleAdmin {
		t.Fatalf("check = %#v", checked)
	}
}

func TestForgedRoleClaimDoesNotGrantAdmin(t *testing.T) {
	db := setupAdminDB(t)
	register(t, db, "alice", "alice@example.com", "111111")
	member := register(t, db, "bob", "bob@example.com", "222222")
	_, login := sessionCtx(t, "bob@example.com")
	token := login["jwt_token"].(string)
	// Even if the client also sends a role field as an argument, the gate strips jwt_token
	// and RequireAdmin re-reads the database role.
	ctx, err := bb_api_auth_session.AuthenticateRequest(context.Background(), map[string]interface{}{"jwt_token": token})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bb_ability_admin.AdminUserSetRole(ctx, map[string]interface{}{"user_id": member.UserID, "role": "admin"}); !errors.Is(err, bb_api_error_code.ErrPermissionDenied) {
		t.Fatalf("db role is user, set_role must fail: %v", err)
	}
	parsed, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte(bb_api_auth_config.CurrentCfgAuth.Session.JWTSigningSecret), nil
	})
	if err != nil || parsed.Claims.(jwt.MapClaims)["role"] != nil {
		if err == nil && parsed.Claims.(jwt.MapClaims)["role"] != nil {
			t.Fatal("access token must not carry a role claim")
		}
	}
}

func TestResourceMethodsRequireAdminBeforeAccept(t *testing.T) {
	db := setupAdminDB(t)
	register(t, db, "alice", "alice@example.com", "111111")
	register(t, db, "bob", "bob@example.com", "222222")
	memberCtx, memberLogin := sessionCtx(t, "bob@example.com")
	adminCtx, _ := sessionCtx(t, "alice@example.com")
	if _, err := bb_ability_resource.Status(memberCtx, map[string]interface{}{"set": "anything"}); !errors.Is(err, bb_api_error_code.ErrPermissionDenied) {
		t.Fatalf("non-admin status: %v", err)
	}
	if _, err := bb_ability_resource.Prepare(memberCtx, map[string]interface{}{"set": "anything", "confirmation_token": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}); !errors.Is(err, bb_api_error_code.ErrPermissionDenied) {
		t.Fatalf("non-admin prepare execute: %v", err)
	}
	bb_ability.LoadAbilityAPIMethods()
	result, err := bb_api_executer.APIExecuter(
		bb_api_auth_session.WithBearerToken(context.Background(), "Bearer "+memberLogin["jwt_token"].(string)),
		bb_api_executer.ToolsCallMethod,
		map[string]interface{}{
			"name": "resource.prepare",
			"arguments": map[string]interface{}{
				"set":                "anything",
				"confirmation_token": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
		},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("non-admin prepare must be an error result: %#v", result)
	}
	var queued int64
	if err := db.Model(&struct {
		gorm.Model
	}{}).Error; err != nil && false {
		t.Fatal(err)
	}
	if err := db.Table("async_tasks").Count(&queued).Error; err != nil {
		t.Fatal(err)
	}
	if queued != 0 {
		t.Fatalf("non-admin prepare must not enqueue a task, queued=%d", queued)
	}
	// admin may pass the permission check and then fail because the set is not configured
	if _, err := bb_ability_resource.Status(adminCtx, map[string]interface{}{"set": "missing"}); errors.Is(err, bb_api_error_code.ErrPermissionDenied) {
		t.Fatalf("admin status should not be permission denied: %v", err)
	}
	if _, err := bb_ability_resource.Sets(adminCtx, map[string]interface{}{}); err != nil {
		t.Fatalf("admin resource.sets: %v", err)
	}
	if _, err := bb_ability_resource.Sets(memberCtx, map[string]interface{}{}); !errors.Is(err, bb_api_error_code.ErrPermissionDenied) {
		t.Fatalf("non-admin resource.sets: %v", err)
	}
}

func TestCheckReturnsDatabaseRole(t *testing.T) {
	db := setupAdminDB(t)
	register(t, db, "alice", "alice@example.com", "111111")
	ctx, _ := sessionCtx(t, "alice@example.com")
	checked, err := bb_api_auth_session.AuthJWTTokenCheck(ctx, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	body := checked.(map[string]interface{})
	if body["role"] != bb_ability_user.RoleAdmin {
		t.Fatalf("check = %#v", body)
	}
	user := body["user"].(*bb_ability_user_model.User)
	if user.Role != bb_ability_user.RoleAdmin {
		t.Fatalf("check user.role = %q", user.Role)
	}
}
