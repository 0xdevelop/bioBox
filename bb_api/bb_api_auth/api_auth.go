// Package api_auth implements protocol-neutral Auth abilities.
package bb_api_auth

import (
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_api_key"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_register"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_session"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_verify_code"
)

func LoadAPIMethods() {
	bb_api_auth_verify_code.LoadAPIMethods()
	bb_api_auth_register.LoadAPIMethods()
	bb_api_auth_session.LoadAPIMethods()
	bb_api_auth_api_key.LoadAPIMethods()
}
