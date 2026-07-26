package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type DatabaseConfig struct {
	Username    string `json:"username" mapstructure:"username"`
	Password    string `json:"password" mapstructure:"password"`
	Host        string `json:"host" mapstructure:"host"`
	Port        int    `json:"port" mapstructure:"port"`
	Dbname      string `json:"dbname" mapstructure:"dbname"`
	Charset     string `json:"charset" mapstructure:"charset"`
	AutoMigrate bool   `json:"auto_migrate" mapstructure:"auto_migrate"`
}

type ServerConfig struct {
	Port string `json:"port" mapstructure:"port"`
	Mode string `json:"mode" mapstructure:"mode"`
}

type OSSConfig struct {
	AccessKeyID     string `json:"access_key_id" mapstructure:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key" mapstructure:"secret_access_key"`
	Bucket          string `json:"bucket" mapstructure:"bucket"`
	Endpoint        string `json:"endpoint" mapstructure:"endpoint"`
	BasePath        string `json:"base_path" mapstructure:"base_path"`
	CDNDomain       string `json:"cdn_domain" mapstructure:"cdn_domain"`
}

type JWTConfig struct {
	SecretKey                 string `json:"secret_key" mapstructure:"secret_key"`
	AccessTokenExpireSeconds  int    `json:"access_token_expire_seconds" mapstructure:"access_token_expire_seconds"`
	RefreshTokenExpireSeconds int    `json:"refresh_token_expire_seconds" mapstructure:"refresh_token_expire_seconds"`
}

type LiveKitConfig struct {
	URL                string `json:"url" mapstructure:"url"`
	APIKey             string `json:"api_key" mapstructure:"api_key"`
	APISecret          string `json:"api_secret" mapstructure:"api_secret"`
	TokenExpireSeconds int    `json:"token_expire_seconds" mapstructure:"token_expire_seconds"`
}

type LogConfig struct {
	Level    string `json:"level" mapstructure:"level"`
	Filename string `json:"filename" mapstructure:"filename"`
}

type ViperConfig struct {
	Server   ServerConfig   `json:"server" mapstructure:"server"`
	Database DatabaseConfig `json:"database" mapstructure:"database"`
	OSS      OSSConfig      `json:"oss" mapstructure:"oss"`
	JWT      JWTConfig      `json:"jwt" mapstructure:"jwt"`
	LiveKit  LiveKitConfig  `json:"livekit" mapstructure:"livekit"`
	Log      LogConfig      `json:"log" mapstructure:"log"`
	Redis    RedisConfig    `json:"redis" mapstructure:"redis"`
}

type RedisConfig struct {
	Addr     string `json:"addr" mapstructure:"addr"`
	Port     int    `json:"port" mapstructure:"port"`
	Password string `json:"password" mapstructure:"password"`
	DB       int    `json:"db" mapstructure:"db"`
}

// LoadConfig reads a config file and applies deployment environment overrides.
func LoadConfig(path string) (*ViperConfig, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("config path is empty")
	}

	reader := viper.New()
	reader.SetConfigFile(path)
	reader.SetConfigType("yaml")
	if err := reader.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	envOverrides := map[string]string{
		"database.password":     "ZAT_DATABASE_PASSWORD",
		"database.auto_migrate": "ZAT_DATABASE_AUTO_MIGRATE",
		"jwt.secret_key":        "ZAT_JWT_SECRET",
		"oss.access_key_id":     "ZAT_OSS_ACCESS_KEY_ID",
		"oss.secret_access_key": "ZAT_OSS_SECRET_ACCESS_KEY",
		"livekit.url":           "ZAT_LIVEKIT_URL",
		"livekit.api_key":       "ZAT_LIVEKIT_API_KEY",
		"livekit.api_secret":    "ZAT_LIVEKIT_API_SECRET",
	}
	for key, envName := range envOverrides {
		if value, ok := os.LookupEnv(envName); ok {
			reader.Set(key, value)
		}
	}

	var config ViperConfig
	if err := reader.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if err := validateConfig(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

func validateConfig(config *ViperConfig) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}
	values := map[string]string{
		"database.password":     config.Database.Password,
		"jwt.secret_key":        config.JWT.SecretKey,
		"oss.access_key_id":     config.OSS.AccessKeyID,
		"oss.secret_access_key": config.OSS.SecretAccessKey,
		"livekit.url":           config.LiveKit.URL,
		"livekit.api_key":       config.LiveKit.APIKey,
		"livekit.api_secret":    config.LiveKit.APISecret,
	}
	for name, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || strings.EqualFold(trimmed, "change-me") || strings.HasPrefix(trimmed, "SET_ZAT_") {
			return fmt.Errorf("config value %s is missing or still a placeholder", name)
		}
	}
	liveKitURL, err := url.Parse(strings.TrimSpace(config.LiveKit.URL))
	if err != nil || liveKitURL.Host == "" {
		return fmt.Errorf("config value livekit.url is invalid")
	}
	switch strings.ToLower(liveKitURL.Scheme) {
	case "http", "https", "ws", "wss":
	default:
		return fmt.Errorf("config value livekit.url must use http, https, ws, or wss")
	}
	if config.LiveKit.TokenExpireSeconds <= 0 {
		return fmt.Errorf("config value livekit.token_expire_seconds must be positive")
	}
	return nil
}

func InitConfig() *ViperConfig {
	config, err := LoadConfig("configs/config.yaml")
	if err != nil {
		panic(fmt.Sprintf("load config failed: %v", err))
	}
	return config
}
