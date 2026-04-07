package db

import (
	"fmt"
	"log/slog"
	"sleet0922/graduation_project/internal/config"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/pkg/logger"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitDB(cfg *config.ViperConfig) *gorm.DB {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		cfg.Database.Username,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Dbname,
		cfg.Database.Charset,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Fatal("连接数据库失败", slog.Any("error", err))
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.Fatal("获取数据库实例失败", slog.Any("error", err))
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	err = db.AutoMigrate(
		&model.User{},
		&model.Friend{},
		&model.FriendRequest{},
		&model.ChatGroup{},
		&model.ChatGroupMember{},
		&model.ChatMessage{},
	)
	if err != nil {
		logger.Fatal("数据库迁移失败", slog.Any("error", err))
	}

	logger.Info("数据库连接成功")
	return db
}
