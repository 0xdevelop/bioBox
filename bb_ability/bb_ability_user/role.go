package bb_ability_user

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_user/bb_ability_user_model"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_config"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_error_code"
	"github.com/0xdevelop/bioBox/bb_storage"
	"gorm.io/gorm"
)

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

func NormalizeRole(raw string) (string, error) {
	role := strings.ToLower(strings.TrimSpace(raw))
	if role != RoleUser && role != RoleAdmin {
		return "", bb_api_error_code.ErrInvalidArguments
	}
	return role, nil
}

func IsAdmin(user *bb_ability_user_model.User) bool {
	return user != nil && user.Role == RoleAdmin
}

// PublicUser 返回可对外展示的用户字段，不含密码哈希等秘密。
func PublicUser(user *bb_ability_user_model.User) map[string]interface{} {
	if user == nil {
		return nil
	}
	item := map[string]interface{}{
		"user_id":           user.UserID,
		"user_name":         user.UserName,
		"nick_name":         user.NickName,
		"bind_email":        user.BindEmail,
		"bind_phone":        user.BindPhone,
		"email_verified_at": formatTime(user.EmailVerifiedAt),
		"phone_verified_at": formatTime(user.PhoneVerifiedAt),
		"role":              displayRole(user.Role),
		"created_at":        user.CreatedAt.UTC().Format(time.RFC3339),
	}
	return item
}

func displayRole(role string) string {
	if role == RoleAdmin {
		return RoleAdmin
	}
	return RoleUser
}

func formatTime(value *time.Time) interface{} {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
}

// BackfillRoles 把空 role 回填为 user。AutoMigrate 增加列后调用。
func BackfillRoles(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	return db.Model(&bb_ability_user_model.User{}).
		Where("role IS NULL OR role = '' OR (role <> ? AND role <> ?)", RoleUser, RoleAdmin).
		Update("role", RoleUser).Error
}

func emailOf(user *bb_ability_user_model.User) string {
	if user == nil || user.BindEmail == nil {
		return ""
	}
	return *user.BindEmail
}

func MatchAdminAllowlist(userName string, email string) bool {
	return bb_api_auth_config.IdentityInAdminList(userName, email)
}

func CountAdmins(tx *gorm.DB) (int64, error) {
	var count int64
	err := tx.Model(&bb_ability_user_model.User{}).Where("role = ?", RoleAdmin).Count(&count).Error
	return count, err
}

// AssignRoleInTx 在同一事务里设置角色；禁止撤销最后一个 admin。
func AssignRoleInTx(tx *gorm.DB, userID string, role string) (*bb_ability_user_model.User, error) {
	role, err := NormalizeRole(role)
	if err != nil || userID == "" {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	user := &bb_ability_user_model.User{}
	if err = tx.Where("user_id = ?", userID).Take(user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, bb_api_error_code.ErrInvalidArguments
		}
		return nil, err
	}
	if user.Role == role {
		return user, nil
	}
	if user.Role == RoleAdmin && role == RoleUser {
		result := tx.Exec(
			`UPDATE users SET role = ? WHERE user_id = ? AND role = ? AND deleted_at IS NULL AND EXISTS (
				SELECT 1 FROM (SELECT 1 FROM users WHERE role = ? AND deleted_at IS NULL AND user_id <> ?) AS other_admin
			)`,
			RoleUser, userID, RoleAdmin, RoleAdmin, userID,
		)
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected != 1 {
			return nil, bb_api_error_code.ErrLastAdmin
		}
		user.Role = RoleUser
		return user, nil
	}
	if err = tx.Model(user).Update("role", role).Error; err != nil {
		return nil, err
	}
	user.Role = role
	return user, nil
}

// TryBootstrapFirstAdmin 仅在配置清单为空、且库中还没有 admin 时，把该用户提升为 admin。
// 使用同一表的条件更新，避免并发注册造出两个首个 admin。
func TryBootstrapFirstAdmin(tx *gorm.DB, userID string) (bool, error) {
	if len(bb_api_auth_config.CurrentAdminIdentities()) > 0 || userID == "" {
		return false, nil
	}
	result := tx.Exec(
		`UPDATE users SET role = ? WHERE user_id = ? AND deleted_at IS NULL AND NOT EXISTS (
			SELECT 1 FROM (SELECT 1 FROM users WHERE role = ? AND deleted_at IS NULL) AS existing_admin
		)`,
		RoleAdmin, userID, RoleAdmin,
	)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// EnsureAllowlistedAdmin 若用户名或邮箱在管理员明单中，则确保其 role 为 admin。
func EnsureAllowlistedAdmin(ctx context.Context, user *bb_ability_user_model.User) (*bb_ability_user_model.User, error) {
	if user == nil || !MatchAdminAllowlist(user.UserName, emailOf(user)) {
		return user, nil
	}
	if user.Role == RoleAdmin {
		return user, nil
	}
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}
	if err = db.Model(&bb_ability_user_model.User{}).Where("user_id = ?", user.UserID).Update("role", RoleAdmin).Error; err != nil {
		return nil, err
	}
	user.Role = RoleAdmin
	return user, nil
}

// ApplyRoleInRegistration 在注册事务内决定新用户角色：明单优先；否则尝试首个 admin 引导。
func ApplyRoleInRegistration(tx *gorm.DB, user *bb_ability_user_model.User) error {
	if user == nil {
		return nil
	}
	if MatchAdminAllowlist(user.UserName, emailOf(user)) {
		if err := tx.Model(user).Update("role", RoleAdmin).Error; err != nil {
			return err
		}
		user.Role = RoleAdmin
		return nil
	}
	became, err := TryBootstrapFirstAdmin(tx, user.UserID)
	if err != nil {
		return err
	}
	if became {
		user.Role = RoleAdmin
	}
	return nil
}

// ApplyConfiguredAdmins 启动时把明单中已存在的账号提升为 admin；不会把名单外的 admin 降权。
func ApplyConfiguredAdmins(ctx context.Context) error {
	identities := bb_api_auth_config.CurrentAdminIdentities()
	if len(identities) == 0 {
		return nil
	}
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		return err
	}
	for _, identity := range identities {
		result := db.Model(&bb_ability_user_model.User{}).
			Where("LOWER(user_name) = ? OR LOWER(bind_email) = ?", identity, identity).
			Update("role", RoleAdmin)
		if result.Error != nil {
			return result.Error
		}
	}
	return nil
}

// PromoteEarliestUserIfNoAdmin 在明单为空且库中还没有 admin 时，把最早的已有用户提升为 admin。
// 用于已有实例升级：避免已有用户全部停在 user，却让下一个新注册账号成为 admin。
func PromoteEarliestUserIfNoAdmin(ctx context.Context) error {
	if len(bb_api_auth_config.CurrentAdminIdentities()) > 0 {
		return nil
	}
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var admins int64
		if err := tx.Model(&bb_ability_user_model.User{}).Where("role = ?", RoleAdmin).Count(&admins).Error; err != nil {
			return err
		}
		if admins > 0 {
			return nil
		}
		first := &bb_ability_user_model.User{}
		if err := tx.Order("id ASC").First(first).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		_, err := TryBootstrapFirstAdmin(tx, first.UserID)
		return err
	})
}

// InitializeRoles 在迁移后回填 role，按配置明单提升已有账号；
// 若明单为空且仍没有 admin，再把最早的已有用户提升为 admin。
func InitializeRoles(ctx context.Context) error {
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		return err
	}
	if err = BackfillRoles(db); err != nil {
		return err
	}
	if err = ApplyConfiguredAdmins(ctx); err != nil {
		return err
	}
	return PromoteEarliestUserIfNoAdmin(ctx)
}
