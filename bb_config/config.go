package bb_config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_config"
	"github.com/0xdevelop/bioBox/bb_frontend/bb_frontend_config"
	"github.com/0xdevelop/bioBox/bb_policy/bb_policy_config"

	"github.com/0xdevelop/bioBox/bb_api/bb_api_config"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_grpc/bb_api_config_grpc"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_jsonRPC/bb_api_config_jsonRPC"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_mcp/bb_api_config_mcp"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_websocket/bb_api_config_websocket"
	"github.com/0xdevelop/bioBox/bb_storage/bb_storage_config"
	"github.com/george012/gtbox/gtbox_log"
	"github.com/goccy/go-json"
	"github.com/goccy/go-yaml"
	"github.com/pelletier/go-toml/v2"

	"github.com/0xdevelop/bioBox/bb_common/bb_app_path"
)

const (
	ProjectName     = "bioBox"
	ProjectVersion  = "v0.0.5"
	ProjectBundleID = "com.bioBox.bioBox"
	// DefaultSessionIssuer / DefaultSessionAudience preserve JWT compatibility with published v0.0.3 clients.
	DefaultSessionIssuer   = "protenix_service"
	DefaultSessionAudience = "protenix_service"
	apiPortDefault         = 7095
)

var (
	GlobalConfig *FileConfig
	HardSN       string
)

type TestUIConfig struct {
	Enabled     bool   `yaml:"enabled" json:"enabled" toml:"enabled"`
	BindAddress string `yaml:"bind_address" json:"bind_address" toml:"bind_address" comment:"监听地址（必填）：127.0.0.1 仅本机；0.0.0.0 对外"`
	Port        int    `yaml:"port" json:"port" toml:"port"`
}

type FileConfig struct {
	StorageCfg  *bb_storage_config.StorageConfig     `yaml:"storage_cfg" json:"storage_cfg" toml:"storage_cfg" comment:"存储"`
	LegacyMysql *bb_storage_config.LegacyMysqlConfig `yaml:"mysql_cfg" json:"mysql_cfg" toml:"mysql_cfg"`
	ApiCfg      *bb_api_config.ApiConfig             `toml:"api_cfg" yaml:"api_cfg" json:"api_cfg" comment:"API configurations"`
	FrontendCfg *bb_frontend_config.FrontendConfig   `yaml:"frontend_cfg" json:"frontend_cfg" toml:"frontend_cfg" comment:"前端页面服务配置"`
	TestUICfg   *TestUIConfig                     `yaml:"test_ui_cfg" json:"test_ui_cfg" toml:"test_ui_cfg" comment:"开发测试台"`
	AuthCfg     *bb_api_auth_config.AuthConfig       `yaml:"auth_cfg" json:"auth_cfg" toml:"auth_cfg" comment:"Auth configurations"`
	PolicyCfg   *bb_policy_config.PolicyConfig       `yaml:"policy_cfg" json:"policy_cfg" toml:"policy_cfg" comment:"Policy scheduling configurations"`
	CliCfg      *CLIConfig                        `yaml:"cli_cfg" json:"cli_cfg" toml:"cli_cfg" comment:"原生 CLI 接线（protenix-run / pxdesign-run）"`
}

func buildYAMLCommentMap(cfg interface{}, parentPath string) yaml.CommentMap {
	commentMap := yaml.CommentMap{}
	val := reflect.ValueOf(cfg)

	// 处理指针类型
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			if parentPath != "" { // 只处理嵌套的nil指针
				val = reflect.New(val.Type().Elem())
			} else {
				return commentMap
			}
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return commentMap
	}

	typ := val.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		if !field.IsExported() {
			continue
		}

		// 解析yaml tag
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "-" {
			continue
		}
		if yamlTag == "" {
			yamlTag = strings.ToLower(field.Name)
		} else if commaIdx := strings.Index(yamlTag, ","); commaIdx != -1 {
			yamlTag = yamlTag[:commaIdx]
		}

		// 修复路径：确保以$开头
		currentPath := parentPath
		if currentPath == "" {
			currentPath = "$." + yamlTag // 根路径
		} else {
			currentPath += "." + yamlTag // 嵌套路径
		}

		// 处理注释
		if comment, ok := field.Tag.Lookup("comment"); ok && comment != "" {
			commentLines := strings.Split(comment, "\n")
			comments := make([]string, 0, len(commentLines))
			for _, line := range commentLines {
				if line != "" {
					comments = append(comments, " "+line)
				}
			}
			commentMap[currentPath] = []*yaml.Comment{
				{
					Texts:    comments,
					Position: yaml.CommentLinePosition,
				},
			}
		}

		// 递归处理嵌套字段
		fieldVal := val.Field(i)
		if !fieldVal.CanInterface() {
			continue
		}

		var nested interface{}
		switch field.Type.Kind() {
		case reflect.Ptr:
			if fieldVal.IsNil() {
				continue
			}
			nested = fieldVal.Interface()
		case reflect.Struct:
			nested = fieldVal.Interface()
		case reflect.Slice:
			if fieldVal.Len() > 0 {
				nested = fieldVal.Index(0).Interface()
			} else if field.Type.Elem().Kind() == reflect.Ptr {
				nested = reflect.New(field.Type.Elem().Elem()).Interface()
			} else {
				nested = reflect.New(field.Type.Elem()).Interface()
			}
		case reflect.Map:
			if fieldVal.Len() > 0 {
				iter := fieldVal.MapRange()
				iter.Next()
				nested = iter.Value().Interface()
			} else {
				nested = reflect.New(field.Type.Elem()).Interface()
			}
		default:
			continue
		}

		nestedComments := buildYAMLCommentMap(nested, currentPath)
		for k, v := range nestedComments {
			commentMap[k] = v
		}
	}
	return commentMap
}

func LoadConfig(file string) error {
	// Decode a fresh snapshot: omitted email/mode fields must not inherit a previous file's delivery route.
	loadedConfig := &FileConfig{}

	buf, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	ext := strings.ToLower(filepath.Ext(file))
	if err := validateAuthDeliveryDocument(buf, ext); err != nil {
		return err
	}
	var decodeErr error
	switch ext {
	case ".yaml", ".yml":
		decodeErr = yaml.Unmarshal(buf, loadedConfig)
	case ".json":
		decodeErr = json.Unmarshal(buf, loadedConfig)
	case ".toml":
		decodeErr = toml.Unmarshal(buf, loadedConfig)
	default:
		if decodeErr = yaml.Unmarshal(buf, loadedConfig); decodeErr != nil {
			if decodeErr = toml.Unmarshal(buf, loadedConfig); decodeErr != nil {
				decodeErr = json.Unmarshal(buf, loadedConfig)
			}
		}
	}
	if decodeErr != nil {
		return decodeErr
	}
	loadedConfig.StorageCfg, err = bb_storage_config.Apply(loadedConfig.StorageCfg, loadedConfig.LegacyMysql)
	if err != nil {
		return err
	}
	if err := loadedConfig.FrontendCfg.Validate(); err != nil {
		return err
	}
	if err := loadedConfig.PolicyCfg.Validate(); err != nil {
		return err
	}
	if err := loadedConfig.CliCfg.Validate(); err != nil {
		return err
	}
	if err := bb_api_auth_config.ValidateVerificationAliases(loadedConfig.AuthCfg); err != nil {
		return err
	}
	if err := bb_api_auth_config.ValidateEmailSelection(loadedConfig.AuthCfg); err != nil {
		return err
	}
	if CurrentApp != nil {
		root, pathErr := bb_app_path.ExecutableDir()
		if pathErr != nil {
			return pathErr
		}
		if loadedConfig.StorageCfg != nil && loadedConfig.StorageCfg.Sqlite != nil {
			sqlite := loadedConfig.StorageCfg.Sqlite
			sqlite.DBFilePath, pathErr = bb_app_path.Resolve(root, sqlite.DBFilePath)
			if pathErr != nil {
				return pathErr
			}
		}
		if loadedConfig.CliCfg != nil {
			if loadedConfig.CliCfg.WorkRoot == "" {
				loadedConfig.CliCfg.WorkRoot = "data/cli_jobs"
			}
			loadedConfig.CliCfg.WorkRoot, pathErr = bb_app_path.Resolve(root, loadedConfig.CliCfg.WorkRoot)
			if pathErr != nil {
				return pathErr
			}
			for _, binary := range []*CLIBinaryConfig{loadedConfig.CliCfg.Protenix, loadedConfig.CliCfg.PXDesign} {
				if binary == nil {
					continue
				}
				binary.Binary, pathErr = bb_app_path.Resolve(root, binary.Binary)
				if pathErr != nil {
					return pathErr
				}
				for _, runtime := range binary.VersionRuntimes {
					if runtime != nil {
						runtime.Binary, pathErr = bb_app_path.Resolve(root, runtime.Binary)
						if pathErr != nil {
							return pathErr
						}
					}
				}
			}
		}
	}
	GlobalConfig = loadedConfig
	bb_api_auth_config.CurrentCfgAuth = loadedConfig.AuthCfg
	bb_policy_config.CurrentCfgPolicy = loadedConfig.PolicyCfg
	return nil
}

func SaveConfig(file string, content *FileConfig) error {
	if file == "" {
		file = CurrentApp.AppConfigFilePath
	}
	// 写入默认配置文件内容
	var err error
	var buf []byte
	// 根据文件扩展名决定使用哪种解析方式
	ext := strings.ToLower(filepath.Ext(file))
	switch ext {
	case ".yaml", ".yml":
		buf, err = yaml.MarshalWithOptions(content, yaml.WithComment(buildYAMLCommentMap(content, "")))
	case ".json":
		buf, err = json.MarshalIndent(content, "", "    ")
	case ".toml":
		buf, err = toml.Marshal(content)
	default:
		// 未知扩展名 → 优先 yaml，回退 toml，最后 json
		buf, err = yaml.MarshalWithOptions(content, yaml.WithComment(buildYAMLCommentMap(content, "")))
		if err != nil {
			if buf2, err2 := toml.Marshal(content); err2 == nil {
				buf, err = buf2, nil
			} else {
				buf, err = json.MarshalIndent(content, "", "    ")
			}
		}
	}

	if err != nil {
		return fmt.Errorf("无法编码配置文件 [%s]: %w", file, err)
	}
	err = os.WriteFile(file, buf, 0755)
	if err != nil {
		return errors.New(fmt.Sprintf("无法写入配置文件 [%s]: %s", file, err.Error()))
	}

	return nil
}

func generateDefaultConfig() *FileConfig {
	authGateEnabledDefault := true
	emailEnabledDefault := false
	sendIntervalDefault := 60
	aport := 13001
	// 生成的缺省配置只监听本机；对外部署由运维改成 0.0.0.0，不在代码里兜底。
	bindLocal := "127.0.0.1"
	fileCfg := &FileConfig{
		StorageCfg: bb_storage_config.Default("./data/bioBox.db"),
		ApiCfg: &bb_api_config.ApiConfig{
			APICfgJsonRPC: &bb_api_config_jsonRPC.APIConfigJsonRPC{
				Enabled:           true,
				BindAddress:       bindLocal,
				Port:              aport,
				EncryptionEnabled: false,
			},
			APICfgMCP: &bb_api_config_mcp.APIConfigMCP{
				Enabled:          true,
				BindAddress:      bindLocal,
				Port:             aport + 1,
				MCPTransportType: bb_api_config_mcp.MCPTransportTypeStreamableHTTP,
			},
			APICfgWebSocket: &bb_api_config_websocket.APIConfigWebSocket{
				Enabled:     true,
				BindAddress: bindLocal,
				Port:        aport + 3,
				AllowedOrigins: []string{
					"127.0.0.1:*",
					"localhost:*",
				},
			},
			APICfgGRPC: &bb_api_config_grpc.APIConfigGRPC{
				Enabled:     true,
				BindAddress: bindLocal,
				Port:        aport + 4,
			},
		},
		FrontendCfg: &bb_frontend_config.FrontendConfig{
			Enabled:     true,
			BindAddress: bindLocal,
			Port:        aport + 5,
		},
		TestUICfg: &TestUIConfig{
			Enabled:     true,
			BindAddress: bindLocal,
			Port:        aport + 2,
		},
		PolicyCfg: &bb_policy_config.PolicyConfig{
			MaxWorkers:                   1,
			PolicyDuration:               "10s",
			WorkersScaller:               1,
			TaskLifecycleTimeoutEnabled:  false,
			TaskExecuteRetryLimitEnabled: false,
		},
		CliCfg: DefaultCLIConfig(),
		AuthCfg: &bb_api_auth_config.AuthConfig{
			Enabled:  &authGateEnabledDefault,
			AuthType: bb_api_auth_config.AuthTypeJWT,
			Email: &bb_api_auth_config.EmailConfig{
				Enabled:             &emailEnabledDefault,
				Provider:            bb_api_auth_config.EmailProviderResend,
				ProductName:         ProjectName,
				VerificationSubject: ProjectName + " verification code",
			},
			Verification: &bb_api_auth_config.VerificationConfig{
				CodeTTLSeconds:      10 * 60,
				MaxAttempts:         5,
				SendIntervalSeconds: &sendIntervalDefault,
				HourlySendLimit:     5,
			},
			Session: &bb_api_auth_config.SessionConfig{
				Issuer:                 DefaultSessionIssuer,
				Audience:               DefaultSessionAudience,
				AccessTokenTTLSeconds:  15 * 60,
				RefreshTokenTTLSeconds: 30 * 24 * 60 * 60,
			},
		},
	}
	return fileCfg
}

func SyncConfigFile(firstRunEnd func(error)) {

	if CurrentApp == nil {
		firstRunEnd(errors.New("App Not Setup "))
		return
	}

	gtbox_log.LogInfof("加载配置文件 [%s]", CurrentApp.AppConfigFilePath)
	_, err := os.Stat(CurrentApp.AppConfigFilePath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		// 获取配置文件的父目录路径
		dir := filepath.Dir(CurrentApp.AppConfigFilePath)

		// 检查父目录是否存在
		if _, err = os.Stat(dir); errors.Is(err, os.ErrNotExist) {
			// 创建父目录
			if err = os.MkdirAll(dir, 0755); err != nil {
				firstRunEnd(errors.New(fmt.Sprintf("无法创建目录 [%s]: %s", dir, err.Error())))
				return
			}
		}

		// 写入默认配置文件内容
		err = SaveConfig(CurrentApp.AppConfigFilePath, generateDefaultConfig())
		if err != nil {
			firstRunEnd(err)
			return
		}
	} else {
		buf, err := os.ReadFile(CurrentApp.AppConfigFilePath)
		if err != nil {
			firstRunEnd(errors.New(fmt.Sprintf("读取配置文件 [%s] 错误: %s", CurrentApp.AppConfigFilePath, err.Error())))
			return
		}
		if len(buf) == 0 {
			gtbox_log.LogErrorf("配置文件重置")
			err = SaveConfig(CurrentApp.AppConfigFilePath, generateDefaultConfig())
			if err != nil {
				firstRunEnd(err)
				return
			}
		}
	}

	err = LoadConfig(CurrentApp.AppConfigFilePath)

	if err != nil {
		firstRunEnd(errors.New(fmt.Sprintf("无法加载配置文件 [%s]: %s", CurrentApp.AppConfigFilePath, err.Error())))
		return
	}

}
