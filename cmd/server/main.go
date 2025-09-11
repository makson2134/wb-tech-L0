package main

import (
    "context"
    "log/slog"
    "net/http"
    _ "net/http/pprof" // Импорт для pprof
    "os"
    "os/signal"
    "runtime"
    "syscall"
    "time"

    "L0/internal/config"
    "L0/internal/repository/postgres"
    "L0/internal/service"
    tHTTP "L0/internal/transport/http"
    "L0/internal/transport/kafka"
    _ "L0/docs" // Импорт для swagger

    "github.com/jackc/pgx/v5/pgxpool"
)

// @title Order API
// @version 1.0
// @description API для работы с заказами
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8081
// @BasePath /
func main() {
    // Json логи по умолчанию
    slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    })))

    cfg := config.MustLoad()

    pool, err := pgxpool.New(context.Background(), cfg.DBDSN)
    if err != nil {
        slog.Error("Failed to connect to database", "error", err)
        os.Exit(1)
    }
    defer pool.Close()

    if err := pool.Ping(context.Background()); err != nil {
        slog.Error("Failed to ping database", "error", err)
        os.Exit(1)
    }

    slog.Info("Connected to database")

    repo := postgres.New(pool, cfg)
    orderService := service.NewOrderService(repo, cfg)

    consumer := kafka.NewConsumer(orderService, cfg)
    orderHandler, err := tHTTP.NewOrderHandler(orderService, "web/template/order.html")
	if err != nil {
    	slog.Error("Failed to create order handler", "error", err)
    	os.Exit(1)
	}
    router := tHTTP.NewRouter(orderHandler, nil, cfg.RateLimiter.RPS, cfg.RateLimiter.Burst, cfg.RateLimiter.Enabled)

    server := &http.Server{
        Addr:         cfg.HTTPAddr,
        Handler:      router,
        ReadTimeout:  cfg.HTTPServer.ReadTimeout,
        WriteTimeout: cfg.HTTPServer.WriteTimeout,
    }

    // Отдельный сервер для pprof
    var pprofServer *http.Server
    if cfg.Monitor.PprofEnabled {
        pprofServer = &http.Server{
            Addr:    cfg.Monitor.PprofAddr,
            Handler: http.DefaultServeMux, // pprof регистрируется в DefaultServeMux
        }
    }

    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()

    go monitorGoroutines(ctx, cfg.Monitor.GoroutinesInterval)
    go consumer.Run(ctx)

    // Запускаем pprof сервер
    if cfg.Monitor.PprofEnabled {
        go func() {
            slog.Info("Starting pprof server", "addr", pprofServer.Addr)
            if err := pprofServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
                slog.Error("pprof server failed", "error", err)
            }
        }()
    }

    // Запускаем основной HTTP сервер
    go func() {
        slog.Info("Starting HTTP server", "addr", cfg.HTTPAddr)
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            slog.Error("HTTP server failed", "error", err)
        }
    }()

    <-ctx.Done()
    slog.Info("Shutting down...")

    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.HTTPServer.ShutdownTimeout)
    defer shutdownCancel()

    // Останавливаем основной сервер
    if err := server.Shutdown(shutdownCtx); err != nil {
        slog.Error("HTTP server shutdown failed", "error", err)
    }

    // Останавливаем pprof сервер
    if cfg.Monitor.PprofEnabled && pprofServer != nil {
        if err := pprofServer.Shutdown(shutdownCtx); err != nil {
            slog.Error("pprof server shutdown failed", "error", err)
        }
    }

    consumer.Close()
    slog.Info("Shutdown complete")
}

func monitorGoroutines(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            slog.Info("Runtime stats",
                "goroutines", runtime.NumGoroutine(),
                "memory_alloc_mb", bToMb(getMemStats().Alloc),
                "memory_sys_mb", bToMb(getMemStats().Sys),
                "gc_cycles", getMemStats().NumGC,
            )
        case <-ctx.Done():
            slog.Info("Stopping goroutine monitor")
            return
        }
    }
}

func getMemStats() runtime.MemStats {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    return m
}

func bToMb(b uint64) uint64 {
    return b / 1024 / 1024
}