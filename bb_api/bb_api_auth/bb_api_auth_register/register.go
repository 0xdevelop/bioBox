package bb_api_auth_register

import (
	"context"
	"errors"
	"time"

	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_user"
	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_user/bb_ability_user_model"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_common"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_config"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_model"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_error_code"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_supported_methods"
	"github.com/0xdevelop/bioBox/bb_storage"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const MethodRegister = "auth.register"

func LoadAPIMethods() {
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{
		Name:        MethodRegister,
		Description: "使用邮箱验证码注册账户；user_name 为注册必填主标识",
		Public:      true,
		InputSchema: bb_api_auth_common.InputSchema(
			map[string]interface{}{
				"user_name":   bb_api_auth_common.StringSchema("", 3, 32),
				"email":       bb_api_auth_common.StringSchema("email", 3, 320),
				"password":    bb_api_auth_common.StringSchema("", 15, 128),
				"verify_code": bb_api_auth_common.VerificationCodeSchema(),
			},
			"user_name",
			"email",
			"password",
			"verify_code",
		),
		Execute: AuthRegister,
	})
}

func AuthRegister(ctx context.Context, input interface{}) (interface{}, error) {
	params, ok := bb_api_auth_common.InputObject(input)
	if !ok || !bb_api_auth_common.HasOnlyKeys(params, "user_name", "email", "password", "verify_code") {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	userNameValue, userNameOK := bb_api_auth_common.RequiredString(params, "user_name")
	emailValue, emailOK := bb_api_auth_common.RequiredString(params, "email")
	password, passwordOK := bb_api_auth_common.RequiredRawString(params, "password")
	code, codeOK := bb_api_auth_common.RequiredString(params, "verify_code")
	if !userNameOK || !emailOK || !passwordOK || !codeOK ||
		!bb_api_auth_common.VerificationCodePattern.MatchString(code) {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	userName, err := bb_ability_user.NormalizeUserName(userNameValue)
	if err != nil {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	email, err := bb_api_auth_common.NormalizeEmail(emailValue)
	if err != nil || bb_ability_user.ValidatePassword(password) != nil {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	return registerWithEmailCode(ctx, userName, email, password, code)
}

func registerWithEmailCode(ctx context.Context, userName string, email string, password string, code string) (map[string]interface{}, error) {
	verificationConfig, err := bb_api_auth_config.CurrentVerificationConfig()
	if err != nil {
		return nil, err
	}
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}
	var registeredUser *bb_ability_user_model.User
	var outcomeErr error
	err = db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		verificationCode := &bb_api_auth_model.AuthVerifyCode{}
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("recipient = ?", email).
			Take(verificationCode).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			outcomeErr = bb_api_error_code.ErrInvalidArguments
			return nil
		}
		if findErr != nil {
			return findErr
		}
		if verificationCode.AttemptsRemaining < 1 {
			outcomeErr = bb_api_error_code.ErrInvalidArguments
			return nil
		}
		if verificationCode.SentAt == nil || verificationCode.CodeHash == "" {
			outcomeErr = bb_api_error_code.ErrInvalidArguments
			return nil
		}
		if !now.Before(verificationCode.ExpiresAt) {
			outcomeErr = bb_api_error_code.ErrInvalidArguments
			return nil
		}

		expectedHash := bb_api_auth_common.VerificationCodeHash(verificationConfig.CodeHashSecret, email, code)
		if !bb_api_auth_common.SecureStringEqual(verificationCode.CodeHash, expectedHash) {
			verificationCode.AttemptsRemaining--
			outcomeErr = bb_api_error_code.ErrInvalidArguments
			if updateErr := tx.Model(verificationCode).
				Update("attempts_remaining", verificationCode.AttemptsRemaining).Error; updateErr != nil {
				return updateErr
			}
			return nil
		}

		user, createErr := bb_ability_user.Create(
			tx,
			userName,
			email,
			password,
			now,
		)
		if createErr != nil {
			if !bb_storage.IsDuplicateKeyError(createErr) {
				return createErr
			}
			outcomeErr = bb_api_error_code.ErrInvalidArguments
		} else if roleErr := bb_ability_user.ApplyRoleInRegistration(tx, user); roleErr != nil {
			return roleErr
		}
		if updateErr := tx.Model(verificationCode).Updates(map[string]interface{}{
			"code_hash": "",
			"sent_at":   nil,
		}).Error; updateErr != nil {
			return updateErr
		}
		if outcomeErr == nil {
			registeredUser = user
		}
		return nil
	})
	if err != nil {
		bb_api_auth_common.LogInternalFailure("register user transaction")
		return nil, err
	}
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	return map[string]interface{}{"user": registeredUser}, nil
}
