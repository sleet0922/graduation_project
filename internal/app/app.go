package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"sleet0922/graduation_project/internal/config"
	dbstore "sleet0922/graduation_project/internal/db"
	"sleet0922/graduation_project/internal/router"
	"sleet0922/graduation_project/pkg/logger"
	redisPkg "sleet0922/graduation_project/pkg/redis"

	"github.com/gofiber/fiber/v2"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Options controls bootstrap dependencies. Factory functions are intentionally
// part of the options so tests can replace network resources with deterministic
// fakes without changing production wiring.
type Options struct {
	ConfigPath string
	Config     *config.ViperConfig

	DB    *gorm.DB
	Redis *goredis.Client

	OpenDB      func(context.Context, *config.ViperConfig) (*gorm.DB, error)
	OpenRedis   func(context.Context, *config.ViperConfig) (*goredis.Client, error)
	SetupLog    func(*config.ViperConfig) (io.Closer, error)
	BuildRouter func(router.Dependencies) (*fiber.App, error)

	// CloseInjectedResources opts into closing DB/Redis values supplied directly
	// by the caller. Resources opened by OpenDB/OpenRedis are always owned.
	CloseInjectedResources bool
	ShutdownTimeout        time.Duration
}

// Application owns the HTTP server and infrastructure clients created during
// bootstrap. Close is idempotent and safe to call from deferred cleanup paths.
type Application struct {
	Config *config.ViperConfig
	HTTP   *fiber.App
	DB     *gorm.DB
	Redis  *goredis.Client

	shutdownTimeout time.Duration
	closers         []func() error
	closeOnce       sync.Once
	closeErr        error
}

// Bootstrap loads configuration, initializes infrastructure, wires the router,
// and returns an application ready for Run. Every failure is returned with
// context; no process exit or panic occurs here.
func Bootstrap(ctx context.Context, options Options) (*Application, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("bootstrap canceled: %w", err)
	}

	cfg := options.Config
	if cfg == nil {
		path := strings.TrimSpace(options.ConfigPath)
		if path == "" {
			path = config.DefaultConfigPath
		}
		loaded, err := config.LoadConfig(path)
		if err != nil {
			return nil, fmt.Errorf("load config: %w", err)
		}
		cfg = loaded
	} else if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	application := &Application{
		Config:          cfg,
		shutdownTimeout: options.ShutdownTimeout,
	}
	if application.shutdownTimeout <= 0 {
		application.shutdownTimeout = 10 * time.Second
	}
	cleanup := func(err error) (*Application, error) {
		if closeErr := application.Close(); closeErr != nil {
			return nil, errors.Join(err, fmt.Errorf("cleanup bootstrap: %w", closeErr))
		}
		return nil, err
	}

	setupLog := options.SetupLog
	if setupLog == nil {
		setupLog = logger.Setup
	}
	logCloser, err := setupLog(cfg)
	if err != nil {
		return cleanup(fmt.Errorf("initialize logger: %w", err))
	}
	if logCloser != nil {
		application.closers = append(application.closers, logCloser.Close)
	}

	database := options.DB
	dbOwned := false
	if database == nil {
		openDB := options.OpenDB
		if openDB == nil {
			openDB = dbstore.Open
		}
		database, err = openDB(ctx, cfg)
		if err != nil {
			return cleanup(fmt.Errorf("initialize database: %w", err))
		}
		if database == nil {
			return cleanup(errors.New("initialize database: factory returned nil database"))
		}
		dbOwned = true
	}
	application.DB = database
	if dbOwned || options.CloseInjectedResources {
		application.closers = append(application.closers, func() error { return dbstore.Close(database) })
	}

	redisClient := options.Redis
	redisOwned := false
	if redisClient == nil {
		openRedis := options.OpenRedis
		if openRedis == nil {
			openRedis = redisPkg.Open
		}
		redisClient, err = openRedis(ctx, cfg)
		if err != nil {
			return cleanup(fmt.Errorf("initialize redis: %w", err))
		}
		if redisClient == nil {
			return cleanup(errors.New("initialize redis: factory returned nil client"))
		}
		redisOwned = true
	}
	application.Redis = redisClient
	if redisOwned || (redisClient != nil && options.CloseInjectedResources) {
		application.closers = append(application.closers, func() error { return redisPkg.Close(redisClient) })
	}

	var dependencies router.Dependencies
	if options.BuildRouter == nil {
		dependencies, err = router.NewDependencies(database, cfg, redisClient)
		if err != nil {
			return cleanup(fmt.Errorf("wire router dependencies: %w", err))
		}
	} else {
		// A custom builder can use only the dependencies it needs (for example a
		// health-only test router), while production uses NewDependencies above.
		dependencies = router.Dependencies{Config: cfg, DB: database, Redis: redisClient}
	}

	buildRouter := options.BuildRouter
	if buildRouter == nil {
		buildRouter = router.NewRouter
	}
	application.HTTP, err = buildRouter(dependencies)
	if err != nil {
		return cleanup(fmt.Errorf("build router: %w", err))
	}
	if application.HTTP == nil {
		return cleanup(errors.New("build router: factory returned nil app"))
	}
	return application, nil
}

// Run serves HTTP until the listener exits or ctx is canceled. Cancellation
// initiates Fiber's graceful shutdown, waits for the listener, then closes all
// owned resources in reverse order.
func (a *Application) Run(ctx context.Context) error {
	if a == nil || a.HTTP == nil {
		return errors.New("application is not initialized")
	}
	if a.Config == nil || strings.TrimSpace(a.Config.Server.Port) == "" {
		return errors.New("application server address is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	// Bind the listener before starting Fiber. Starting HTTP.Listen in a
	// goroutine leaves a cancellation window where Shutdown can run before the
	// listener exists, allowing the server to start after Run has returned.
	network := a.HTTP.Config().Network
	if strings.TrimSpace(network) == "" {
		network = fiber.NetworkTCP4
	}
	listener, err := net.Listen(network, a.Config.Server.Port)
	if err != nil {
		closeErr := a.Close()
		return errors.Join(fmt.Errorf("listen http: %w", err), closeErr)
	}
	defer func() { _ = listener.Close() }()

	listenErr := make(chan error, 1)
	go func() {
		listenErr <- a.HTTP.Listener(listener)
	}()

	select {
	case err := <-listenErr:
		closeErr := a.Close()
		if err != nil {
			return errors.Join(fmt.Errorf("serve http: %w", err), closeErr)
		}
		return closeErr
	case <-ctx.Done():
		// Close the pre-bound listener first. This handles cancellation that
		// arrives while Fiber is still preparing its server internals.
		var listenerErr error
		if err := listener.Close(); err != nil && !isExpectedShutdownError(err) {
			listenerErr = fmt.Errorf("close http listener: %w", err)
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
		shutdownErr := a.HTTP.ShutdownWithContext(shutdownCtx)
		cancel()

		var serverErr error
		timer := time.NewTimer(a.shutdownTimeout)
		select {
		case err := <-listenErr:
			if err != nil && !isExpectedShutdownError(err) {
				serverErr = fmt.Errorf("serve http: %w", err)
			}
		case <-timer.C:
			serverErr = fmt.Errorf("serve http: listener did not stop within %s", a.shutdownTimeout)
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		closeErr := a.Close()
		if shutdownErr != nil && !isServerNotRunning(shutdownErr) && !isExpectedShutdownError(shutdownErr) {
			shutdownErr = fmt.Errorf("shutdown http: %w", shutdownErr)
		} else {
			shutdownErr = nil
		}
		return errors.Join(serverErr, shutdownErr, listenerErr, closeErr)
	}
}

func isExpectedShutdownError(err error) bool {
	return err == nil || strings.Contains(strings.ToLower(err.Error()), "server closed") || strings.Contains(strings.ToLower(err.Error()), "use of closed network connection")
}

func isServerNotRunning(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "server is not running")
}

// Close releases owned resources in reverse initialization order. It does not
// call HTTP.Shutdown; Run handles that while the listener is still active.
func (a *Application) Close() error {
	if a == nil {
		return nil
	}
	a.closeOnce.Do(func() {
		for i := len(a.closers) - 1; i >= 0; i-- {
			if err := a.closers[i](); err != nil {
				a.closeErr = errors.Join(a.closeErr, err)
			}
		}
	})
	return a.closeErr
}

// Shutdown gracefully stops the HTTP server and then closes owned resources.
// It is useful to callers that manage the listener themselves; Run invokes the
// same operations when its context is canceled.
func (a *Application) Shutdown(ctx context.Context) error {
	if a == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var shutdownErr error
	if a.HTTP != nil {
		shutdownErr = a.HTTP.ShutdownWithContext(ctx)
		if isServerNotRunning(shutdownErr) || isExpectedShutdownError(shutdownErr) {
			shutdownErr = nil
		}
	}
	return errors.Join(shutdownErr, a.Close())
}
