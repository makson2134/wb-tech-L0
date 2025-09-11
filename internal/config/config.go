package config

import (
    "log/slog"
    "os"
    "time"

    "github.com/ilyakaznacheev/cleanenv"
    "github.com/joho/godotenv"
)

type Config struct {
    DBDSN       string `env:"DB_DSN" env-default:"postgres://orders_user:orders_password@postgres:5432/orders?sslmode=disable"`
    HTTPAddr    string `env:"HTTP_ADDR" env-default:":8081"`
    HTTPServer  `env-prefix:"HTTP_"`
    DB          `env-prefix:"DB_"`
    Kafka       `env-prefix:"KAFKA_"`
    Cache       `env-prefix:"CACHE_"`
    RateLimiter `env-prefix:"RATE_LIMITER_"`
    Producer    `env-prefix:"PRODUCER_"`
    Monitor     `env-prefix:"MONITOR_"`
    Retry       `env-prefix:"RETRY_"`
}

type HTTPServer struct {
    ReadTimeout     time.Duration `env:"READ_TIMEOUT" env-default:"30s"`
    WriteTimeout    time.Duration `env:"WRITE_TIMEOUT" env-default:"30s"`
    ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" env-default:"30s"`
}

type DB struct {
    Host    string `env:"HOST" env-required:""`
    Port    string `env:"PORT" env-required:""`
    User    string `env:"USER" env-required:""`
    Pass    string `env:"PASSWORD" env-required:""`
    Name    string `env:"NAME" env-required:""`
    SSLMode string `env:"SSLMODE" env-default:"disable"`
}

type Kafka struct {
    Brokers       []string      `env:"BROKERS" env-required:"true" env-separator:","`
    Topic         string        `env:"TOPIC" env-required:"true"`
    CommitTimeout time.Duration `env:"COMMIT_TIMEOUT" env-default:"10s"`
}

type Cache struct {
    Size int           `env:"SIZE" env-default:"1000"`
    TTL  time.Duration `env:"TTL" env-default:"30m"`
}

type RateLimiter struct {
    RPS     float64 `env:"RPS" env-default:"10"`
    Burst   int     `env:"BURST" env-default:"20"`
    Enabled bool    `env:"ENABLED" env-default:"true"`
}

type Producer struct {
    DataPath string        `env:"DATA_PATH" env-default:"testdata"`
    Delay    time.Duration `env:"DELAY" env-default:"2s"`
}

type Monitor struct {
    GoroutinesInterval time.Duration `env:"GOROUTINES_INTERVAL" env-default:"30s"`
    PprofEnabled       bool          `env:"PPROF_ENABLED" env-default:"true"`
    PprofAddr          string        `env:"PPROF_ADDR" env-default:":6060"`
    PprofWebPort       string        `env:"PPROF_WEB_PORT" env-default:"8080"`
    PprofCPUDuration   time.Duration `env:"PPROF_CPU_DURATION" env-default:"30s"`
}

type Retry struct {
    MaxElapsedTimeDB   time.Duration `env:"MAX_ELAPSED_TIME_DB" env-default:"5s"`
    MaxElapsedTimeRead time.Duration `env:"MAX_ELAPSED_TIME_READ" env-default:"3s"`
    InitialInterval    time.Duration `env:"INITIAL_INTERVAL" env-default:"100ms"`
    MaxIntervalDB      time.Duration `env:"MAX_INTERVAL_DB" env-default:"1s"`
    MaxIntervalRead    time.Duration `env:"MAX_INTERVAL_READ" env-default:"500ms"`
}

func MustLoad() *Config {
    // Для локальной разработки подгружаем .env файл
    if err := godotenv.Load(); err != nil {
        slog.Info("No .env file found, reading from environment variables")
    }

    var cfg Config

    if err := cleanenv.ReadEnv(&cfg); err != nil {
        slog.Error("cannot read config from environment", "error", err)
        os.Exit(1)
    }

    return &cfg
}