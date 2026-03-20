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
	Port         string `json:"port"`
	Mode_Debug   string `json:"mode_debug"`
	Mode_Release string `json:"mode_release"`
}

type Config struct {
	Server   GinPortConfig  `json:"server"`
	Database DatabaseConfig `json:"database"`
}

func InitConfig() *Config {
	viper.SetConfigFile("configs/config.yaml")
	viper.SetConfigType("yaml")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Sprintf("读取配置文件失败: %v", err))
	}

	var config Config
	err = viper.Unmarshal(&config)
	if err != nil {
		panic(fmt.Sprintf("解析配置文件失败: %v", err))
	}

	return &config
}
