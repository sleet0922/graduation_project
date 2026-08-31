package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

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
	Port           string `json:"port" mapstructure:"port"`
	Mode           string `json:"mode" mapstructure:"mode"`
	AllowedOrigins string `json:"allowed_origins" mapstructure:"allowed_origins"`
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

const DefaultConfigPath = "configs/config.yaml"

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
		"server.allowed_origins": "ZAT_SERVER_ALLOWED_ORIGINS",
		"database.password":      "ZAT_DATABASE_PASSWORD",
		"database.auto_migrate":  "ZAT_DATABASE_AUTO_MIGRATE",
		"jwt.secret_key":         "ZAT_JWT_SECRET",
		"oss.access_key_id":      "ZAT_OSS_ACCESS_KEY_ID",
		"oss.secret_access_key":  "ZAT_OSS_SECRET_ACCESS_KEY",
		"livekit.url":            "ZAT_LIVEKIT_URL",
		"livekit.api_key":        "ZAT_LIVEKIT_API_KEY",
		"livekit.api_secret":     "ZAT_LIVEKIT_API_SECRET",
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
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &config, nil
}

// Validate checks a fully decoded configuration before infrastructure is
// opened. Keeping validation on the value makes explicit-config bootstrap and
// file-based bootstrap follow the same contract.
func (config *ViperConfig) Validate() error {
	return validateConfig(config)
}

func validateConfig(config *ViperConfig) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}
	if err := validateDeploymentMode(); err != nil {
		return err
	}
	if err := validateServerMode(config.Server.Mode); err != nil {
		return err
	}
	if err := validateListenAddress(config.Server.Port); err != nil {
		return fmt.Errorf("config value server.port: %w", err)
	}
	if err := validateHost("database.host", config.Database.Host); err != nil {
		return err
	}
	if err := validateIdentifier("database.username", config.Database.Username); err != nil {
		return err
	}
	if err := validateIdentifier("database.dbname", config.Database.Dbname); err != nil {
		return err
	}
	if err := validatePort("database.port", config.Database.Port); err != nil {
		return err
	}
	if err := validateHost("redis.addr", config.Redis.Addr); err != nil {
		return err
	}
	if err := validatePort("redis.port", config.Redis.Port); err != nil {
		return err
	}
	if err := validateLogFilename(config.Log.Filename); err != nil {
		return err
	}
	if err := validateLogLevel(config.Log.Level); err != nil {
		return err
	}
	if config.Redis.DB < 0 || config.Redis.DB > 15 {
		return fmt.Errorf("config value redis.db must be between 0 and 15 (got %d)", config.Redis.DB)
	}
	values := []struct {
		name  string
		value string
	}{
		{name: "database.password", value: config.Database.Password},
		{name: "jwt.secret_key", value: config.JWT.SecretKey},
		{name: "oss.access_key_id", value: config.OSS.AccessKeyID},
		{name: "oss.secret_access_key", value: config.OSS.SecretAccessKey},
		{name: "livekit.url", value: config.LiveKit.URL},
		{name: "livekit.api_key", value: config.LiveKit.APIKey},
		{name: "livekit.api_secret", value: config.LiveKit.APISecret},
	}
	for _, item := range values {
		name, value := item.name, item.value
		trimmed := strings.TrimSpace(value)
		lower := strings.ToLower(trimmed)
		if trimmed == "" || lower == "change-me" || strings.HasPrefix(lower, "set_zat_") {
			return fmt.Errorf("config value %s is missing or still a placeholder", name)
		}
		if hasControl(trimmed) {
			return fmt.Errorf("config value %s contains control characters", name)
		}
		if !allowInsecureDefaults() && (strings.HasPrefix(lower, "local-development-") || lower == "devkey") {
			return fmt.Errorf("config value %s uses an insecure development default", name)
		}
	}
	if err := validateLiveKitURL(config.LiveKit.URL); err != nil {
		return err
	}
	if config.LiveKit.TokenExpireSeconds <= 0 {
		return fmt.Errorf("config value livekit.token_expire_seconds must be positive")
	}
	if config.JWT.AccessTokenExpireSeconds <= 0 {
		return fmt.Errorf("config value jwt.access_token_expire_seconds must be positive")
	}
	if config.JWT.RefreshTokenExpireSeconds <= 0 {
		return fmt.Errorf("config value jwt.refresh_token_expire_seconds must be positive")
	}
	if config.JWT.RefreshTokenExpireSeconds < config.JWT.AccessTokenExpireSeconds {
		return fmt.Errorf("config value jwt.refresh_token_expire_seconds must be >= access_token_expire_seconds")
	}
	if err := validateAllowedOrigins(config.Server.AllowedOrigins); err != nil {
		return err
	}
	return nil
}

func allowInsecureDefaults() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("BOCKER_DEPLOYMENT_MODE")), "development") && os.Getenv("ZAT_ALLOW_INSECURE_DEFAULTS") == "1"
}

func validateDeploymentMode() error {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("BOCKER_DEPLOYMENT_MODE")))
	if mode == "" || mode == "development" || mode == "production" {
		return nil
	}
	return fmt.Errorf("BOCKER_DEPLOYMENT_MODE must be development or production (got %q)", mode)
}

func validateServerMode(mode string) error {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "debug", "release", "test", "development", "production":
		return nil
	default:
		return fmt.Errorf("config value server.mode must be debug, release, test, development, or production (got %q)", mode)
	}
}

func validateHost(name, raw string) error {
	host := strings.TrimSpace(raw)
	if host == "" {
		return fmt.Errorf("config value %s is missing", name)
	}
	if hasControlOrSpace(host) || strings.ContainsAny(host, "/\\") {
		return fmt.Errorf("config value %s contains invalid characters", name)
	}
	// IPv6 values may be supplied with or without brackets in YAML. Reject
	// unmatched brackets instead of silently normalizing malformed input.
	if strings.HasPrefix(host, "[") || strings.HasSuffix(host, "]") {
		if len(host) < 2 || host[0] != '[' || host[len(host)-1] != ']' {
			return fmt.Errorf("config value %s is not a valid host", name)
		}
		host = host[1 : len(host)-1]
	}
	if net.ParseIP(host) != nil {
		return nil
	}
	if strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") || strings.Contains(host, "..") {
		return fmt.Errorf("config value %s is not a valid host", name)
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return fmt.Errorf("config value %s is not a valid host", name)
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
				return fmt.Errorf("config value %s is not a valid host", name)
			}
		}
	}
	return nil
}

func validateIdentifier(name, raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fmt.Errorf("config value %s is missing", name)
	}
	if len(value) > 63 || hasControlOrSpace(value) {
		return fmt.Errorf("config value %s contains invalid characters", name)
	}
	// The container entrypoint creates the role and database using unquoted
	// PostgreSQL identifiers. Restrict the config to that same grammar so an
	// otherwise valid-looking config cannot fail later during initialization.
	for index, r := range value {
		if index == 0 {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && r != '_' {
				return fmt.Errorf("config value %s must start with a letter or underscore", name)
			}
			continue
		}
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return fmt.Errorf("config value %s contains invalid characters", name)
		}
	}
	return nil
}

func validateLogFilename(raw string) error {
	filename := strings.TrimSpace(raw)
	if filename == "" {
		return fmt.Errorf("config value log.filename is missing")
	}
	if hasControlOrSpace(filename) || filepath.Clean(filename) == "." || strings.HasSuffix(filename, string(filepath.Separator)) {
		return fmt.Errorf("config value log.filename is invalid")
	}
	if info, err := os.Stat(filename); err == nil && info.IsDir() {
		return fmt.Errorf("config value log.filename points to a directory")
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("config value log.filename cannot be inspected: %w", err)
	}
	return nil
}

func validateLogLevel(raw string) error {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "debug", "info", "warn", "warning", "error":
		return nil
	default:
		return fmt.Errorf("config value log.level is invalid (got %q)", raw)
	}
}

func validateAllowedOrigins(raw string) error {
	for _, item := range strings.Split(raw, ",") {
		origin := strings.TrimSpace(item)
		if origin == "" || origin == "*" {
			continue
		}
		u, err := url.Parse(origin)
		if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("config value server.allowed_origins contains invalid origin %q", origin)
		}
		if !strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https") {
			return fmt.Errorf("config value server.allowed_origins contains unsupported scheme in %q", origin)
		}
	}
	return nil
}

func validateLiveKitURL(raw string) error {
	if raw == "" || hasControlOrSpace(raw) {
		return fmt.Errorf("config value livekit.url is invalid")
	}
	value := strings.TrimSpace(raw)
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" {
		return fmt.Errorf("config value livekit.url is invalid")
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "ws", "wss":
	default:
		return fmt.Errorf("config value livekit.url must use http, https, ws, or wss")
	}
	hostname := u.Hostname()
	if err := validateHost("livekit.url.host", hostname); err != nil {
		return err
	}
	// url.Parse rejects alphabetic ports, but it treats a trailing colon as an
	// empty port and accepts numeric values outside the TCP range. Validate the
	// authority explicitly so malformed endpoints fail at startup.
	if strings.HasSuffix(u.Host, ":") {
		return fmt.Errorf("config value livekit.url contains an empty port")
	}
	portText := ""
	if strings.HasPrefix(u.Host, "[") {
		closeBracket := strings.LastIndex(u.Host, "]")
		if closeBracket < 0 {
			return fmt.Errorf("config value livekit.url has an invalid IPv6 authority")
		}
		if len(u.Host) > closeBracket+1 {
			if u.Host[closeBracket+1] != ':' {
				return fmt.Errorf("config value livekit.url has an invalid authority")
			}
			portText = u.Host[closeBracket+2:]
		}
	} else if strings.Count(u.Host, ":") == 1 {
		portText = u.Host[strings.LastIndexByte(u.Host, ':')+1:]
	} else if strings.Count(u.Host, ":") > 1 {
		return fmt.Errorf("config value livekit.url must bracket an IPv6 host")
	}
	if portText != "" {
		port, convErr := strconv.Atoi(portText)
		if convErr != nil || port < 1 || port > 65535 {
			return fmt.Errorf("config value livekit.url port is invalid: %q", portText)
		}
	}
	return nil
}

func hasControlOrSpace(value string) bool {
	return strings.IndexFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0
}

func hasControl(value string) bool {
	return strings.IndexFunc(value, unicode.IsControl) >= 0
}

func validatePort(name string, port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("config value %s must be between 1 and 65535 (got %d)", name, port)
	}
	return nil
}

func validateListenAddress(raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fmt.Errorf("is missing")
	}
	host, portText, err := net.SplitHostPort(value)
	if err != nil {
		return fmt.Errorf("must be host:port (got %q)", raw)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return fmt.Errorf("port %q is invalid", portText)
	}
	if err := validatePort("server.port", port); err != nil {
		return err
	}
	if host != "" {
		if err := validateHost("server.host", host); err != nil {
			return err
		}
	}
	return nil
}

func InitConfig() (*ViperConfig, error) {
	return LoadConfig(DefaultConfigPath)
}
