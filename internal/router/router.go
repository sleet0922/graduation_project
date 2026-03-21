package router

import (
	"sleet0922/graduation_project/internal/handler"
	"sleet0922/graduation_project/internal/repo"
	"sleet0922/graduation_project/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func InitRouter(db *gorm.DB) *gin.Engine {
	r := gin.Default()

	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	userRepo := repo.NewUserRepository(db)
	userService := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userService)

	r.POST("/api/user/add", userHandler.Add)
	r.POST("/api/delete_all", userHandler.DeleteAll)
	r.POST("/api/add_test_user", userHandler.AddTestUser)

	return r
}
