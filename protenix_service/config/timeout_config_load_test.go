package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xdevelop/protenix_service/policy/policy_config"
)

func TestLoadConfigTaskLifecycleTimeoutExplicitSwitch(t *testing.T) {
	previous := GlobalConfig
	previousPolicy := policy_config.CurrentCfgPolicy
	t.Cleanup(func() {
		GlobalConfig = previous
		policy_config.CurrentCfgPolicy = previousPolicy
	})
	base := "storage_cfg:\n  storage_sqlite:\n    enabled: true\n    path: ./data/t.db\n"
	for _, tc := range []struct {
		name    string
		policy  string
		wantErr bool
		want    bool
	}{
		{"enabled false ignores zero", "policy_cfg:\n  task_lifecycle_timeout_enabled: false\n  task_lifecycle_timeout_seconds: 0\n", false, false},
		{"enabled false ignores negative", "policy_cfg:\n  task_lifecycle_timeout_enabled: false\n  task_lifecycle_timeout_seconds: -7\n", false, false},
		{"enabled true zero rejected", "policy_cfg:\n  task_lifecycle_timeout_enabled: true\n  task_lifecycle_timeout_seconds: 0\n", true, false},
		{"enabled true negative rejected", "policy_cfg:\n  task_lifecycle_timeout_enabled: true\n  task_lifecycle_timeout_seconds: -1\n", true, false},
		{"enabled true positive", "policy_cfg:\n  task_lifecycle_timeout_enabled: true\n  task_lifecycle_timeout_seconds: 60\n", false, true},
		{"cli execute timeout false ignores zero", "cli_cfg:\n  execute_timeout_enabled: false\n  execute_timeout_seconds: 0\n", false, false},
		{"cli execute timeout true zero rejected", "cli_cfg:\n  execute_timeout_enabled: true\n  execute_timeout_seconds: 0\n", true, false},
		{"cli execute timeout true negative rejected", "cli_cfg:\n  execute_timeout_enabled: true\n  execute_timeout_seconds: -3\n", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			GlobalConfig = &FileConfig{}
			policy_config.CurrentCfgPolicy = nil
			file := filepath.Join(t.TempDir(), "c.yaml")
			if err := os.WriteFile(file, []byte(base+tc.policy), 0o600); err != nil {
				t.Fatal(err)
			}
			err := LoadConfig(file)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected validation error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if policy_config.TaskLifecycleTimeoutEnforced() != tc.want {
				t.Fatalf("enforced = %t, want %t", policy_config.TaskLifecycleTimeoutEnforced(), tc.want)
			}
		})
	}
}

func TestCLIExecuteTimeoutExplicitSwitch(t *testing.T) {
	for _, seconds := range []int{-1, 0, 5, 3600} {
		cfg := &CLIConfig{ExecuteTimeoutEnabled: false, ExecuteTimeoutSeconds: seconds}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("enabled=false seconds=%d must pass: %v", seconds, err)
		}
		if cfg.CurrentExecuteTimeout() != 0 {
			t.Fatalf("enabled=false seconds=%d must be ignored", seconds)
		}
	}
	cfg := &CLIConfig{ExecuteTimeoutEnabled: true, ExecuteTimeoutSeconds: 5}
	if err := cfg.Validate(); err != nil || cfg.CurrentExecuteTimeout() != 5*time.Second {
		t.Fatalf("enabled=true seconds=5: err=%v timeout=%s", err, cfg.CurrentExecuteTimeout())
	}
	if def := DefaultCLIConfig(); def.ExecuteTimeoutEnabled || def.CurrentExecuteTimeout() != 0 {
		t.Fatal("default cli_cfg must have execute_timeout_enabled=false")
	}
	var nilCfg *CLIConfig
	if nilCfg.Validate() != nil || nilCfg.CurrentExecuteTimeout() != 0 {
		t.Fatal("nil cli_cfg must be off")
	}
}
