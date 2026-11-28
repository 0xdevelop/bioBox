// Package main (bioBox)
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_user"
	"github.com/0xdevelop/bioBox/bb_api"
	"github.com/0xdevelop/bioBox/bb_common"
	"github.com/0xdevelop/bioBox/bb_common/bb_app_path"
	"github.com/0xdevelop/bioBox/bb_config"
	"github.com/0xdevelop/bioBox/bb_custom_cmd"
	"github.com/0xdevelop/bioBox/bb_frontend"
	"github.com/0xdevelop/bioBox/bb_policy"
	"github.com/0xdevelop/bioBox/bb_storage"
	"github.com/0xdevelop/bioBox/bb_test_ui"
	"github.com/george012/gtbox"
	"github.com/george012/gtbox/gtbox_cmd"
	"github.com/george012/gtbox/gtbox_log"
)

var (
	mRunMode       = ""
	mGitCommitHash = ""
	mGitCommitTime = ""
	mPackageOS     = ""
	mPackageTime   = ""
	mGoVersion     = ""
)

func setupApp() {
	runMode := gtbox.RunModeDebug
	switch mRunMode {
	case "debug":
		runMode = gtbox.RunModeDebug
	case "test":
		runMode = gtbox.RunModeTest
	case "release":
		runMode = gtbox.RunModeRelease
	default:
		runMode = gtbox.RunModeDebug
	}

	bb_config.CurrentApp = bb_config.NewApp(
		bb_config.ProjectName,
		bb_config.ProjectBundleID,
		fmt.Sprintf("%s service",
			bb_config.ProjectName,
		),
		runMode,
	)

	if bb_config.CurrentApp.CurrentRunMode == gtbox.RunModeDebug {
		cmdMap := map[string]string{
			"git_commit_hash": "git show -s --format=%H",
			"git_commit_time": "git show -s --format=\"%ci\" | cut -d ' ' -f 1,2 | sed 's/ /_/'",
			"build_os":        "go env GOOS",
			"go_version":      "go version | awk '{print $3}'",
		}
		cmdRes := gtbox_cmd.RunWith(cmdMap)

		if cmdRes != nil {
			mGitCommitHash = cmdRes["git_commit_hash"]
			mGitCommitTime = cmdRes["git_commit_time"]
			mPackageOS = cmdRes["build_os"]
			mGoVersion = cmdRes["go_version"]
			mPackageTime = time.Now().UTC().Format("2006-01-02_15:04:05")
		}
	}

	root, err := bb_app_path.ExecutableDir()
	if err != nil {
		panic("cannot resolve executable resource root")
	}
	bb_config.CurrentApp.AppRunAsDir = root
	bb_config.CurrentApp.AppLogPath = filepath.Join(root, "logs")
	bb_config.CurrentApp.AppConfigFilePath = filepath.Join(root, "conf/config.yaml")

	bb_config.CurrentApp.GitCommitHash = mGitCommitHash
	bb_config.CurrentApp.GitCommitTime = mGitCommitTime
	bb_config.CurrentApp.GoVersion = mGoVersion
	bb_config.CurrentApp.PackageOS = mPackageOS
	bb_config.CurrentApp.PackageTime = mPackageTime

	bb_custom_cmd.HandleCustomCmds(os.Args, bb_config.CurrentApp)

	gtbox.SetupGTBox(bb_config.CurrentApp.AppName,
		bb_config.CurrentApp.CurrentRunMode,
		bb_config.CurrentApp.AppLogPath,
		30,
		gtbox_log.GTLogSaveHours,
		bb_config.CurrentApp.HTTPRequestTimeOut,
	)
}

func main() {
	runtime.LockOSThread()

	setupApp()

	defer bb_common.PanicHandler()

	// debug 读 CWD 相对 ./example_files/config_local.yaml，供 GoLand 远程调试使用。
	// test/release 沿用 setupApp 已算出的 ExecutableDir()/conf/config.yaml，
	// 不再用 CWD 相对 ./conf/config.yaml 覆盖。
	bb_config.CurrentApp.AppConfigFilePath = configFilePathForRunMode(
		bb_config.CurrentApp.CurrentRunMode,
		bb_config.CurrentApp.AppConfigFilePath,
	)
	bb_config.SyncConfigFile(func(err error) {
		if err != nil {
			gtbox_log.LogErrorf("配置加载失败，进程终止：%v", err)
			os.Exit(1)
		}
	})
	if bb_config.GlobalConfig == nil {
		gtbox_log.LogErrorf("配置加载后 GlobalConfig 为空，进程终止")
		os.Exit(1)
	}

	if err := bb_storage.OpenStorage(); err != nil {
		gtbox_log.LogErrorf("storage open failed: %v", err)
		return
	}
	defer func() {
		if err := bb_storage.CloseStorage(); err != nil {
			gtbox_log.LogErrorf("storage close failed: %v", err)
		}
	}()
	if err := bb_storage.GormAutoMigrate(); err != nil {
		gtbox_log.LogErrorf("gorm AutoMigrate failed: %v", err)
		return
	}
	if err := bb_ability_user.InitializeRoles(context.Background()); err != nil {
		gtbox_log.LogErrorf("initialize user roles failed: %v", err)
		return
	}
	gtbox_log.LogInfof("storage migrated")

	if err := bb_config.GlobalConfig.ApiCfg.Validate(); err != nil {
		gtbox_log.LogErrorf("api config invalid: %v; refusing to start", err)
		return
	}
	bb_api.StartAPIServices(bb_config.GlobalConfig.ApiCfg)
	stopPolicy := bb_policy.PolicyServicesStart()
	defer stopPolicy()
	defer bb_api.StopApiServices()
	if bb_config.TestUIEnabled() {
		bb_test_ui.LoadTestWeb()
	}
	if bb_config.FrontendEnabled() {
		frontendServer, err := bb_frontend.FrontendServicesStart(bb_config.GlobalConfig.FrontendCfg)
		if err != nil {
			gtbox_log.LogErrorf("frontend start failed: %v", err)
			return
		}
		if frontendServer != nil {
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := frontendServer.Shutdown(ctx); err != nil {
					gtbox_log.LogErrorf("frontend shutdown failed: %v", err)
				}
			}()
		}
	}
	bb_common.LoadSigHandle(nil, nil)

}
