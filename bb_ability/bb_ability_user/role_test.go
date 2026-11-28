package bb_ability_user

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_user/bb_ability_user_model"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_config"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_error_code"
	"github.com/0xdevelop/bioBox/bb_config"
	"github.com/0xdevelop/bioBox/bb_storage"
	"github.com/0xdevelop/bioBox/bb_storage/bb_storage_config"
	"gorm.io/gorm"
)

const roleTestPassword = "correct-horse-battery-staple"

func setupRoleDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousConfig := bb_config.GlobalConfig
	previousAuth := bb_api_auth_config.CurrentCfgAuth
	bb_config.GlobalConfig = &bb_config.FileConfig{StorageCfg: bb_storage_config.Default(filepath.Join(t.TempDir(), "roles.db"))}
	bb_api_auth_config.CurrentCfgAuth = &bb_api_auth_config.AuthConfig{}
	t.Cleanup(func() {
		_ = bb_storage.CloseStorage()
		bb_config.GlobalConfig = previousConfig
		bb_api_auth_config.CurrentCfgAuth = previousAuth
	})
	if err := bb_storage.OpenStorage(); err != nil {
		t.Fatalf("OpenStorage: %v", err)
	}
	if err := bb_storage.GormAutoMigrate(); err != nil {
		t.Fatalf("GormAutoMigrate: %v", err)
	}
	if err := InitializeRoles(context.Background()); err != nil {
		t.Fatalf("InitializeRoles: %v", err)
	}
	db, err := bb_storage.GormDB(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func createInTx(t *testing.T, db *gorm.DB, name, email string) *bb_ability_user_model.User {
	t.Helper()
	var user *bb_ability_user_model.User
	if err := db.Transaction(func(tx *gorm.DB) error {
		created, err := Create(tx, name, email, roleTestPassword, time.Now().UTC())
		if err != nil {
			return err
		}
		if err = ApplyRoleInRegistration(tx, created); err != nil {
			return err
		}
		user = created
		return nil
	}); err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	return user
}

func TestFirstCreatedUserBecomesAdminWhenAllowlistEmpty(t *testing.T) {
	db := setupRoleDB(t)
	first := createInTx(t, db, "alice", "alice@example.com")
	if first.Role != RoleAdmin {
		t.Fatalf("first user role = %q", first.Role)
	}
	second := createInTx(t, db, "bob", "bob@example.com")
	if second.Role != RoleUser {
		t.Fatalf("second user role = %q", second.Role)
	}
}

func TestAllowlistPromotesMatchingAccountsAndSkipsOthers(t *testing.T) {
	db := setupRoleDB(t)
	bb_api_auth_config.CurrentCfgAuth.AdminUsers = []string{"Boss", "owner@example.com"}
	outsider := createInTx(t, db, "carol", "carol@example.com")
	if outsider.Role != RoleUser {
		t.Fatal("when allowlist is set, a non-listed first user must stay user")
	}
	listed := createInTx(t, db, "boss", "boss@example.com")
	if listed.Role != RoleAdmin {
		t.Fatal("allowlisted username must become admin on registration assignment")
	}
	created, err := Create(db, "owneracct", "owner@example.com", roleTestPassword, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if created.Role != RoleUser {
		t.Fatalf("direct create should default to user, got %q", created.Role)
	}
	promoted, err := EnsureAllowlistedAdmin(context.Background(), created)
	if err != nil || promoted.Role != RoleAdmin {
		t.Fatalf("allowlist ensure: %+v err %v", promoted, err)
	}
	if err := db.Model(&bb_ability_user_model.User{}).Where("user_id = ?", created.UserID).Update("role", RoleUser).Error; err != nil {
		t.Fatal(err)
	}
	if err := ApplyConfiguredAdmins(context.Background()); err != nil {
		t.Fatal(err)
	}
	reloaded, err := FindByUserID(db, created.UserID)
	if err != nil || reloaded.Role != RoleAdmin {
		t.Fatalf("startup allowlist promote: %+v err %v", reloaded, err)
	}
}

func TestConcurrentFirstAdminBootstrapCreatesOnlyOne(t *testing.T) {
	db := setupRoleDB(t)
	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			name := "user" + string(rune('a'+i))
			errs[i] = db.Transaction(func(tx *gorm.DB) error {
				user, err := Create(tx, name, name+"@example.com", roleTestPassword, time.Now().UTC())
				if err != nil {
					return err
				}
				return ApplyRoleInRegistration(tx, user)
			})
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d: %v", i, err)
		}
	}
	var admins int64
	if err := db.Model(&bb_ability_user_model.User{}).Where("role = ?", RoleAdmin).Count(&admins).Error; err != nil {
		t.Fatal(err)
	}
	if admins != 1 {
		t.Fatalf("expected exactly one first admin, got %d", admins)
	}
}

func TestLastAdminCannotBeDemoted(t *testing.T) {
	db := setupRoleDB(t)
	first := createInTx(t, db, "alice", "alice@example.com")
	err := db.Transaction(func(tx *gorm.DB) error {
		_, err := AssignRoleInTx(tx, first.UserID, RoleUser)
		return err
	})
	if !errors.Is(err, bb_api_error_code.ErrLastAdmin) {
		t.Fatalf("demoting last admin: %v", err)
	}
	second := createInTx(t, db, "bob", "bob@example.com")
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, err := AssignRoleInTx(tx, second.UserID, RoleAdmin)
		return err
	}); err != nil {
		t.Fatalf("promote second admin: %v", err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, err := AssignRoleInTx(tx, first.UserID, RoleUser)
		return err
	}); err != nil {
		t.Fatalf("demote one of two admins should work: %v", err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, err := AssignRoleInTx(tx, second.UserID, RoleUser)
		return err
	}); !errors.Is(err, bb_api_error_code.ErrLastAdmin) {
		t.Fatalf("demoting remaining admin: %v", err)
	}
}

func TestBackfillEmptyRolesToUser(t *testing.T) {
	db := setupRoleDB(t)
	user, err := Create(db, "dave", "dave@example.com", roleTestPassword, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(user).Update("role", "").Error; err != nil {
		t.Fatal(err)
	}
	if err := BackfillRoles(db); err != nil {
		t.Fatal(err)
	}
	reloaded, err := FindByUserID(db, user.UserID)
	if err != nil || reloaded.Role != RoleUser {
		t.Fatalf("backfill = %+v err %v", reloaded, err)
	}
}

func TestPublicUserOmitsPasswordHash(t *testing.T) {
	user := &bb_ability_user_model.User{UserID: "u", UserName: "alice", NickName: "Alice", PasswordHash: "secret", Role: RoleAdmin}
	pub := PublicUser(user)
	if _, exists := pub["password_hash"]; exists || pub["role"] != RoleAdmin {
		t.Fatalf("public user = %#v", pub)
	}
}

func TestInitializeRolesPromotesEarliestExistingUserWhenNoAdmin(t *testing.T) {
	db := setupRoleDB(t)
	first, err := Create(db, "early", "early@example.com", roleTestPassword, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	second, err := Create(db, "later", "later@example.com", roleTestPassword, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if first.Role != RoleUser || second.Role != RoleUser {
		t.Fatalf("direct create should stay user, got %q and %q", first.Role, second.Role)
	}
	if err := InitializeRoles(context.Background()); err != nil {
		t.Fatal(err)
	}
	reloadedFirst, err := FindByUserID(db, first.UserID)
	if err != nil || reloadedFirst.Role != RoleAdmin {
		t.Fatalf("earliest existing user should become admin, got %+v err %v", reloadedFirst, err)
	}
	reloadedSecond, err := FindByUserID(db, second.UserID)
	if err != nil || reloadedSecond.Role != RoleUser {
		t.Fatalf("later user must stay user, got %+v err %v", reloadedSecond, err)
	}
}

func TestApplyConfiguredAdminsMatchesIgnoreCase(t *testing.T) {
	db := setupRoleDB(t)
	user, err := Create(db, "BossUser", "Owner.Admin@example.com", roleTestPassword, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	bb_api_auth_config.CurrentCfgAuth.AdminUsers = []string{"bossuser"}
	if err := ApplyConfiguredAdmins(context.Background()); err != nil {
		t.Fatal(err)
	}
	reloaded, err := FindByUserID(db, user.UserID)
	if err != nil || reloaded.Role != RoleAdmin {
		t.Fatalf("case-insensitive username allowlist: %+v err %v", reloaded, err)
	}
	if err := db.Model(user).Update("role", RoleUser).Error; err != nil {
		t.Fatal(err)
	}
	bb_api_auth_config.CurrentCfgAuth.AdminUsers = []string{"owner.admin@example.com"}
	if err := ApplyConfiguredAdmins(context.Background()); err != nil {
		t.Fatal(err)
	}
	reloaded, err = FindByUserID(db, user.UserID)
	if err != nil || reloaded.Role != RoleAdmin {
		t.Fatalf("case-insensitive email allowlist: %+v err %v", reloaded, err)
	}
}
