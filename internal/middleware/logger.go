package middleware

import (
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"sleet0922/graduation_project/pkg/errcode"
	"sleet0922/graduation_project/pkg/logger"
	"sleet0922/graduation_project/pkg/response"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		c.Next()
		cost := time.Since(start)
		logger.Info(path,
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("ip", c.ClientIP()),
			zap.Duration("cost", cost),
			zap.String("query", query),
			zap.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()),
		)
	}
}

func isError(err interface{}) bool {
	if ne, ok := err.(*net.OpError); ok {
		if se, ok := ne.Err.(*os.SyscallError); ok {
			errStr := strings.ToLower(se.Error())
			return strings.Contains(errStr, "broken pipe") || strings.Contains(errStr, "connection reset by peer")
		}
	}
	return false
}

func GinRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			err := recover()
			if err != nil {
				req, _ := httputil.DumpRequest(c.Request, false)
				if isError(err) {
					logger.Error(c.Request.URL.Path, zap.Any("error", err), zap.String("request", string(req)))
					if e, ok := err.(error); ok {
						c.Error(e)
					}
					c.Abort()
					return
				}
				logger.Error("[Recovery from panic]",
					zap.Any("error", err),
					zap.String("request", string(req)),
					zap.String("stack", string(debug.Stack())),
				)
				response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
				c.Abort()
			}
		}()
		c.Next()
	}
}
