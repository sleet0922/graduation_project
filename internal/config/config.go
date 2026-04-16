package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type DatabaseConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Dbname   string `json:"dbname"`
	Charset  string `json:"charset"`
}

type GinPortConfig struct {
	Port     string `json:"port" mapstructure:"port"`
	Mode     string `json:"mode" mapstructure:"mode"`
	CertFile string `json:"cert_file" mapstructure:"cert_file"`
	KeyFile  string `json:"key_file" mapstructure:"key_file"`
}

type OSSConfig struct {
	AccessKeyID     string `json:"access_key_id" mapstructure:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key" mapstructure:"secret_access_key"`
	Bucket          string `json:"bucket" mapstructure:"bucket"`
	Endpoint        string `json:"endpoint" mapstructure:"endpoint"`
	BasePath        string `json:"base_path" mapstructure:"base_path"`
}

type JWTConfig struct {
	SecretKey string `json:"secret_key" mapstructure:"secret_key"`
}

type RTCConfig struct {
	AppID              string `json:"app_id" mapstructure:"app_id"`
	AppKey             string `json:"app_key" mapstructure:"app_key"`
	TokenExpireSeconds int    `json:"token_expire_seconds" mapstructure:"token_expire_seconds"`
}

type LogConfig struct {
	Level    string `json:"level" mapstructure:"level"`
	Filename string `json:"filename" mapstructure:"filename"`
}

type ViperConfig struct {
	Server   GinPortConfig  `json:"server"`
	Database DatabaseConfig `json:"database"`
	OSS      OSSConfig      `json:"oss" mapstructure:"oss"`
	JWT      JWTConfig      `json:"jwt" mapstructure:"jwt"`
	RTC      RTCConfig      `json:"rtc" mapstructure:"rtc"`
	Log      LogConfig      `json:"log" mapstructure:"log"`
	Redis    RedisConfig    `json:"redis" mapstructure:"redis"`
}
type RedisConfig struct {
	Addr     string `json:"addr"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	DB       int    `json:"db"`
}

func InitConfig() *ViperConfig {
	viper.SetConfigFile("configs/config.yaml")
	viper.SetConfigType("yaml")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Sprintf("读取配置文件失败: %v", err))
	}

	var config ViperConfig
	err = viper.Unmarshal(&config)
	if err != nil {
		panic(fmt.Sprintf("解析配置文件失败: %v", err))
	}

	return &config
}
