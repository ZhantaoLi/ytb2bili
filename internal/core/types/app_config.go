package types

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// AppConfig 应用程序配置
type AppConfig struct {
	Path        string        `toml:"-"`
	Listen      string        `toml:"listen"`
	Environment string        `toml:"environment"`
	Debug       bool          `toml:"debug"`
	Database    Database      `toml:"database"`
	Auth        AuthConfig    `toml:"auth"`
	AppAuth     AppAuthConfig `toml:"app_auth"` // 应用启动认证配置
	APIAuth     APIAuthConfig `toml:"api_auth"` // API请求认证配置（go-auth）
	FileUpDir   string        `toml:"fileUpDir"`
	DataPath    string        `toml:"data_path"`   // 数据存储路径（用于 cookies 等）
	YtDlpPath   string        `toml:"yt_dlp_path"` // yt-dlp 安装路径
	AutoUpload  bool          `toml:"auto_upload"` // 是否自动上传准备就绪的视频到Bilibili（false=仅准备到就绪态，需手动上传）

	PrimaryAIService       string                  `toml:"primary_ai_service"`     // 首选AI服务提供商
	TenCosConfig           *TencentCosConfig       `toml:"TenCosConfig"`           // 腾讯云 COS 存储配置
	OpenAICompatibleConfig *OpenAICompatibleConfig `toml:"OpenAICompatibleConfig"` // OpenAI兼容API配置
	DeepLXConfig           *DeepLXConfig           `toml:"DeepLXConfig"`           // DeepLX 翻译配置
	BaiduTransConfig       *BaiduTransConfig       `toml:"BaiduTransConfig"`       // 百度翻译服务配置
	DeepSeekTransConfig    *DeepSeekTransConfig    `toml:"DeepSeekTransConfig"`    // DeepSeek翻译服务配置
	GeminiConfig           *GeminiConfig           `toml:"GeminiConfig"`           // Gemini多模态服务配置
	TranslatorConfig       *TranslatorConfig       `toml:"TranslatorConfig"`       // 翻译器总配置
	ProxyConfig            *ProxyConfig            `toml:"ProxyConfig"`            // 代理配置
	AnalyticsConfig        *AnalyticsConfig        `toml:"AnalyticsConfig"`        // 数据分析配置
	BilibiliConfig         *BilibiliConfig         `toml:"BilibiliConfig"`         // Bilibili上传配置
	WhisperConfig          *WhisperConfig          `toml:"WhisperConfig"`          // Whisper 语音识别配置
}

// BilibiliConfig Bilibili上传配置
type BilibiliConfig struct {
	Copyright           int    `toml:"copyright"`             // 1=自制, 2=转载
	Source              string `toml:"source"`                // 转载来源（当 Copyright=2 时必填）
	NoReprint           int    `toml:"no_reprint"`            // 0=允许转载, 1=禁止转载
	UseOriginalTitle    bool   `toml:"use_original_title"`    // true=使用原视频标题, false=使用AI生成标题
	UseOriginalDesc     bool   `toml:"use_original_desc"`     // true=使用原视频描述, false=使用AI生成描述
	CustomTitleTemplate string `toml:"custom_title_template"` // 自定义标题模板，支持变量: {original_title}, {ai_title}
	CustomDescTemplate  string `toml:"custom_desc_template"`  // 自定义描述模板，支持变量: {original_desc}, {ai_desc}

	// 新增配置项
	Tid              int    `toml:"tid"`                // 分区ID（默认122，可自定义）
	Dynamic          string `toml:"dynamic"`            // 动态文本（默认"发布了新视频！"）
	OpenElec         int    `toml:"open_elec"`          // 是否开启充电面板 0=关闭, 1=开启
	SelectionReserve int64  `toml:"selection_reserve"`  // 参与活动ID（0表示不参与）
	UpSelectionReply int    `toml:"up_selection_reply"` // 是否展示推荐评论 0=关闭, 1=开启
	UpCloseReply     int    `toml:"up_close_reply"`     // 是否关闭评论 0=开启评论, 1=关闭评论
	UpCloseReward    int    `toml:"up_close_reward"`    // 是否关闭打赏 0=开启, 1=关闭
}

type TencentCosConfig struct {
	Enabled      bool // 是否启用腾讯云 COS 存储
	CosBucketURL string
	CosSecretId  string
	CosSecretKey string
	CosRegion    string
	CosBucket    string
	SubAppId     string
	CosUrL       string
}

// Database 数据库配置
type Database struct {
	Type     string `toml:"type"`     // postgres, mysql, sqlite
	Host     string `toml:"host"`     // 对于 sqlite，这是数据库文件路径
	Port     int    `toml:"port"`     // sqlite 不需要
	Username string `toml:"username"` // sqlite 不需要
	Password string `toml:"password"` // sqlite 不需要
	Database string `toml:"database"` // 对于 sqlite，这是文件名
	SSLMode  string `toml:"ssl_mode"` // sqlite 不需要
	Timezone string `toml:"timezone"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	JWTSecret     string `toml:"jwt_secret"`
	JWTExpiration int    `toml:"jwt_expiration"` // 小时
	SessionSecret string `toml:"session_secret"`
}

// AppAuthConfig 应用启动认证配置
type AppAuthConfig struct {
	Enabled       bool   `toml:"enabled"`        // 是否启用应用认证
	APIURL        string `toml:"api_url"`        // 认证API地址
	AppID         string `toml:"app_id"`         // 应用ID
	AppSecret     string `toml:"app_secret"`     // 应用密钥
	CheckInterval int    `toml:"check_interval"` // 定期检查间隔（分钟），0表示只在启动时检查
	SkipOnError   bool   `toml:"skip_on_error"`  // 认证失败时是否跳过（开发环境可设置为true）
}

// APIAuthConfig API请求认证配置（使用go-auth进行签名验证）
type APIAuthConfig struct {
	AppID             string `toml:"app_id"`              // 应用ID
	AppSecret         string `toml:"app_secret"`          // 应用密钥
	CookiesDecryptKey string `toml:"cookies_decrypt_key"` // Cookies 解密密钥
}

// GetDSN 获取数据库连接字符串
func (d Database) GetDSN() string {
	switch d.Type {
	case "postgres", "postgresql":
		return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
			d.Host, d.Username, d.Password, d.Database, d.Port, d.SSLMode, d.Timezone)
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			d.Username, d.Password, d.Host, d.Port, d.Database)
	case "sqlite", "sqlite3":
		// SQLite 数据库文件路径
		if d.Host != "" {
			// 如果指定了 host，使用 host 作为完整路径
			return d.Host
		}
		// 否则使用 database 作为文件名，存储在当前目录
		if d.Database == "" {
			d.Database = filepath.Join("data", "ytb2bili.db")
		}
		return d.Database
	default:
		return ""
	}
}

// BaiduTransConfig 百度翻译服务配置
type BaiduTransConfig struct {
	Enabled   bool   `toml:"enabled"`    // 是否启用翻译服务
	AppId     string `toml:"app_id"`     // 百度翻译AppID
	SecretKey string `toml:"secret_key"` // 百度翻译密钥
	Endpoint  string `toml:"endpoint"`   // API端点
}

// DeepSeekTransConfig DeepSeek翻译服务配置
type DeepSeekTransConfig struct {
	Enabled   bool   `toml:"enabled"`    // 是否启用翻译服务
	ApiKey    string `toml:"api_key"`    // DeepSeek API密钥
	Model     string `toml:"models"`     // 使用的模型，默认为 deepseek-chat
	Endpoint  string `toml:"endpoint"`   // API端点，默认为 https://api.deepseek.com
	Timeout   int    `toml:"timeout"`    // 超时时间（秒）
	MaxTokens int    `toml:"max_tokens"` // 最大token数
}

// DeepLXConfig DeepLX 翻译服务配置
type DeepLXConfig struct {
	Enabled    bool   `toml:"enabled"`     // 是否启用 DeepLX 翻译
	Endpoint   string `toml:"endpoint"`    // DeepLX /translate 完整端点
	SourceLang string `toml:"source_lang"` // 源语言，默认 EN
	TargetLang string `toml:"target_lang"` // 目标语言，默认 ZH
	Timeout    int    `toml:"timeout"`     // 超时时间（秒）
}

// GeminiConfig Gemini多模态服务配置
type GeminiConfig struct {
	Enabled           bool   `toml:"enabled"`             // 是否启用Gemini服务
	ApiKey            string `toml:"api_key"`             // Google AI API密钥
	Model             string `toml:"model"`               // 使用的模型，默认为 gemini-1.5-pro
	Timeout           int    `toml:"timeout"`             // 超时时间（秒）
	MaxTokens         int    `toml:"max_tokens"`          // 最大输出token数
	UseForMetadata    bool   `toml:"use_for_metadata"`    // 是否使用Gemini生成元数据（优先于DeepSeek）
	AnalyzeVideo      bool   `toml:"analyze_video"`       // 是否分析视频文件（true=多模态，false=仅文本）
	VideoSampleFrames int    `toml:"video_sample_frames"` // 视频采样帧数（0=上传完整视频）
}

// TranslatorConfig 翻译器总配置
type TranslatorConfig struct {
	DefaultProvider   string   `toml:"default_provider"`   // 默认翻译提供商
	FallbackProviders []string `toml:"fallback_providers"` // 备选翻译提供商
	MaxRetries        int      `toml:"max_retries"`        // 最大重试次数
	Timeout           int      `toml:"timeout"`            // 超时时间（秒）
	EnableCache       bool     `toml:"enable_cache"`       // 是否启用缓存
	CacheExpiry       int      `toml:"cache_expiry"`       // 缓存过期时间（秒）
}

// ProxyConfig 代理配置
type ProxyConfig struct {
	UseProxy  bool   `toml:"use_proxy"`  // 是否使用代理
	ProxyHost string `toml:"proxy_host"` // 代理地址 (例如: http://127.0.0.1:7890)
}

// AnalyticsConfig 数据分析配置
type AnalyticsConfig struct {
	Enabled       bool   `toml:"enabled"`        // 是否启用数据分析
	ServerURL     string `toml:"server_url"`     // 分析服务器地址
	APIKey        string `toml:"api_key"`        // API密钥
	ProductID     string `toml:"product_id"`     // 产品ID
	Debug         bool   `toml:"debug"`          // 是否启用调试模式
	EncryptionKey string `toml:"encryption_key"` // AES加密密钥（可选，16/24/32字节）
}

// WhisperConfig Whisper 语音识别配置
type WhisperConfig struct {
	Enabled   bool   `toml:"enabled"`    // 是否启用 Whisper
	ModelPath string `toml:"model_path"` // Whisper 模型文件路径
	Language  string `toml:"language"`   // 识别语言 (en, zh, auto等)
	Threads   int    `toml:"threads"`    // 使用的线程数
}

// OpenAICompatibleConfig OpenAI兼容API配置
type OpenAICompatibleConfig struct {
	Enabled     bool    `toml:"enabled"`     // 是否启用
	Provider    string  `toml:"provider"`    // 提供商: openai, deepseek, qwen, zhipu, gemini等
	APIKey      string  `toml:"api_key"`     // API密钥
	BaseURL     string  `toml:"base_url"`    // API基础URL
	Model       string  `toml:"model"`       // 使用的模型
	Timeout     int     `toml:"timeout"`     // 超时时间（秒）
	MaxTokens   int     `toml:"max_tokens"`  // 最大token数
	Temperature float64 `toml:"temperature"` // 温度参数（0-2）
}

// NewDefaultConfig 创建默认配置
func NewDefaultConfig() *AppConfig {
	return &AppConfig{
		Listen:      ":8096",
		Environment: "development",
		Debug:       true,
		FileUpDir:   "./data", // 默认任务文件目录
		DataPath:    "./data", // 默认数据存储路径
		YtDlpPath:   "",
		AutoUpload:  false, // 默认关闭自动上传：视频准备到就绪态后停在 200，由用户手动上传
		Database: Database{
			Type:     "sqlite",
			Database: filepath.Join("data", "ytb2bili.db"),
			Timezone: "Asia/Shanghai",
		},

		Auth: AuthConfig{
			JWTSecret:     "",
			JWTExpiration: 24,
			SessionSecret: "your-session-secret",
		},

		// API 认证配置默认关闭；如需启用，请在 config.toml 中自行配置。
		APIAuth: APIAuthConfig{
			AppID:             "",
			AppSecret:         "",
			CookiesDecryptKey: "",
		},

		// 首选AI服务提供商（空表示自动选择）
		PrimaryAIService: "",

		// 腾讯云 COS 配置（默认值，可被 config.toml 覆盖）
		TenCosConfig: &TencentCosConfig{
			Enabled:      false,
			CosBucketURL: "",
			CosSecretId:  "",
			CosSecretKey: "",
			CosRegion:    "",
			CosBucket:    "",
			SubAppId:     "",
			CosUrL:       "",
		},

		// DeepSeek 翻译配置（默认值，可被 config.toml 覆盖）
		DeepSeekTransConfig: &DeepSeekTransConfig{
			Enabled:   false,
			ApiKey:    "",
			Model:     "deepseek-chat",
			Endpoint:  "https://api.deepseek.com",
			Timeout:   60,
			MaxTokens: 4000,
		},

		// DeepLX 翻译配置默认关闭；如需启用，请在 config.toml 中自行配置 endpoint。
		DeepLXConfig: &DeepLXConfig{
			Enabled:    false,
			Endpoint:   "",
			SourceLang: "EN",
			TargetLang: "ZH",
			Timeout:    30,
		},

		// Gemini 多模态配置（默认值，可被 config.toml 覆盖）
		GeminiConfig: &GeminiConfig{
			Enabled:           false,
			ApiKey:            "",
			Model:             "gemini-2.5-flash",
			Timeout:           120,
			MaxTokens:         8000,
			UseForMetadata:    false, // 默认不启用，优先使用DeepSeek
			AnalyzeVideo:      true,  // 默认启用视频分析
			VideoSampleFrames: 0,     // 默认上传完整视频
		},

		// 代理配置（默认值，可被 config.toml 覆盖）
		ProxyConfig: &ProxyConfig{
			UseProxy:  false,
			ProxyHost: "",
		},

		// 数据分析配置（默认值，可被 config.toml 覆盖）
		AnalyticsConfig: &AnalyticsConfig{
			Enabled:   false,
			ServerURL: "http://localhost:8080",
			APIKey:    "",
			ProductID: "bili-up-api",
			Debug:     false,
		},

		// Bilibili 配置（默认值，可被 config.toml 覆盖）
		BilibiliConfig: &BilibiliConfig{
			Copyright:          1, // 默认自制
			Source:             "",
			NoReprint:          1,         // 默认禁止转载
			UseOriginalTitle:   false,     // 默认使用生成/翻译后的中文标题
			UseOriginalDesc:    false,     // 默认使用生成/翻译后的中文简介
			CustomDescTemplate: "",        // 默认不使用自定义模板
			Tid:                122,       // 默认分区：日常
			Dynamic:            "发布了新视频！", // 默认动态
			OpenElec:           0,         // 默认关闭充电
			SelectionReserve:   0,         // 默认不参与活动
			UpSelectionReply:   0,         // 默认不展示推荐评论
			UpCloseReply:       0,         // 默认开启评论
			UpCloseReward:      1,         // 默认关闭打赏
		},

		// // OpenAI 兼容 API 配置（默认值，可被 config.toml 覆盖）
		OpenAICompatibleConfig: &OpenAICompatibleConfig{
			Enabled:     false,
			Provider:    "local",
			APIKey:      "",
			BaseURL:     "http://127.0.0.1:11434/v1",
			Model:       "qwen2.5:7b",
			Timeout:     60,
			MaxTokens:   4000,
			Temperature: 0.7,
		},
	}
}

// LoadConfig 加载配置
func LoadConfig(configFile string) (*AppConfig, error) {
	// 先创建默认配置（包含所有硬编码的配置）
	config := NewDefaultConfig()
	config.Path = configFile

	// 检查配置文件是否存在
	_, err := os.Stat(configFile)
	if err != nil {
		// 如果文件不存在，不创建默认配置文件，使用硬编码配置即可
		return config, nil
	}

	if _, err := toml.DecodeFile(configFile, config); err != nil {
		return nil, err
	}
	return config, nil
}

// SaveConfig 保存配置（只保存可配置字段，不保存硬编码配置）
func SaveConfig(config *AppConfig) error {
	// 只保存用户可配置的字段
	fileConfig := struct {
		Listen                 string                  `toml:"listen"`
		Environment            string                  `toml:"environment"`
		Debug                  bool                    `toml:"debug"`
		Database               Database                `toml:"database"`
		Auth                   AuthConfig              `toml:"auth"`
		APIAuth                APIAuthConfig           `toml:"api_auth"`
		FileUpDir              string                  `toml:"fileUpDir"`
		DataPath               string                  `toml:"data_path"`
		YtDlpPath              string                  `toml:"yt_dlp_path"`
		AutoUpload             bool                    `toml:"auto_upload"`
		PrimaryAIService       string                  `toml:"primary_ai_service"`
		TenCosConfig           *TencentCosConfig       `toml:"TenCosConfig"`
		OpenAICompatibleConfig *OpenAICompatibleConfig `toml:"OpenAICompatibleConfig"`
		DeepLXConfig           *DeepLXConfig           `toml:"DeepLXConfig"`
		DeepSeekTransConfig    *DeepSeekTransConfig    `toml:"DeepSeekTransConfig"`
		GeminiConfig           *GeminiConfig           `toml:"GeminiConfig"`
		ProxyConfig            *ProxyConfig            `toml:"ProxyConfig"`
		AnalyticsConfig        *AnalyticsConfig        `toml:"AnalyticsConfig"`
		BilibiliConfig         *BilibiliConfig         `toml:"BilibiliConfig"`
		WhisperConfig          *WhisperConfig          `toml:"WhisperConfig"`
	}{
		Listen:                 config.Listen,
		Environment:            config.Environment,
		Debug:                  config.Debug,
		Database:               config.Database,
		Auth:                   config.Auth,
		APIAuth:                config.APIAuth,
		FileUpDir:              config.FileUpDir,
		DataPath:               config.DataPath,
		YtDlpPath:              config.YtDlpPath,
		AutoUpload:             config.AutoUpload,
		PrimaryAIService:       config.PrimaryAIService,
		TenCosConfig:           config.TenCosConfig,
		OpenAICompatibleConfig: config.OpenAICompatibleConfig,
		DeepLXConfig:           config.DeepLXConfig,
		DeepSeekTransConfig:    config.DeepSeekTransConfig,
		GeminiConfig:           config.GeminiConfig,
		ProxyConfig:            config.ProxyConfig,
		AnalyticsConfig:        config.AnalyticsConfig,
		BilibiliConfig:         config.BilibiliConfig,
		WhisperConfig:          config.WhisperConfig,
	}

	buf := new(bytes.Buffer)

	// 写入注释说明
	buf.WriteString("# Bilibili 视频上传后端 - 配置文件\n\n")
	buf.WriteString("# 注意：以下配置已硬编码在代码中，无需在此配置：\n")
	buf.WriteString("# - BaiduTransConfig (百度翻译)\n")
	buf.WriteString("# - app_auth (应用认证)\n")
	buf.WriteString("# \n")
	buf.WriteString("# 所有配置都可以通过 config.toml 或 API 接口动态配置\n\n")

	encoder := toml.NewEncoder(buf)
	if err := encoder.Encode(&fileConfig); err != nil {
		return err
	}

	return os.WriteFile(config.Path, buf.Bytes(), 0600)
}
