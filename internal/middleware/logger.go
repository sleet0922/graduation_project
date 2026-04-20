package middleware

import (
	"log/slog"
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
)

func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		c.Next()
		cost := time.Since(start)
		logger.Info(path,
			slog.Int("status", c.Writer.Status()),
			slog.String("method", c.Request.Method),
			slog.String("ip", c.ClientIP()),
			slog.Duration("cost", cost),
			slog.String("query", query),
			slog.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()),
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
					logger.Error(c.Request.URL.Path, slog.Any("error", err), slog.String("request", string(req)))
					if e, ok := err.(error); ok {
						_ = c.Error(e)
					}
					c.Abort()
					return
				}
				logger.Error("[Recovery from panic]",
					slog.Any("error", err),
					slog.String("request", string(req)),
					slog.String("stack", string(debug.Stack())),
				)
				response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
				c.Abort()
			}
		}()
		c.Next()
	}
}
