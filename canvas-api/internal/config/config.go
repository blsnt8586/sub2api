package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Config 是 canvas-api 的完整运行配置。全部来自环境变量（或 .env）。
type Config struct {
	Host               string
	Port               string
	DB                 DBConfig
	JWTSecret          string
	CORSAllowedOrigins []string
	Wasabi             WasabiConfig
}

// DBConfig 描述与 sub2api 共享的 PostgreSQL 连接参数。
type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// DSN 返回 lib/pq 风格的连接串。
func (d DBConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode)
}

// WasabiConfig 是 S3 兼容对象存储配置（Phase 3 起用）。
type WasabiConfig struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
}

// Enabled 报告 Wasabi 是否已配置齐全。
func (w WasabiConfig) Enabled() bool {
	return w.Bucket != "" && w.AccessKey != "" && w.SecretKey != ""
}

// Load 读取 .env（若存在）后从环境变量组装配置，并校验必填项。
func Load() (*Config, error) {
	loadDotEnv(".env")

	cfg := &Config{
		Host: env("CANVAS_API_HOST", "0.0.0.0"),
		Port: env("CANVAS_API_PORT", "8081"),
		DB: DBConfig{
			Host:     env("DATABASE_HOST", "127.0.0.1"),
			Port:     env("DATABASE_PORT", "5434"),
			User:     env("DATABASE_USER", "sub2api"),
			Password: env("DATABASE_PASSWORD", ""),
			DBName:   env("DATABASE_DBNAME", "sub2api"),
			SSLMode:  env("DATABASE_SSLMODE", "disable"),
		},
		JWTSecret: env("JWT_SECRET", ""),
		Wasabi: WasabiConfig{
			Endpoint:  env("WASABI_ENDPOINT", ""),
			Region:    env("WASABI_REGION", ""),
			Bucket:    env("WASABI_BUCKET", ""),
			AccessKey: env("WASABI_ACCESS_KEY", ""),
			SecretKey: env("WASABI_SECRET_KEY", ""),
		},
	}

	origins := env("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:3001")
	for _, o := range strings.Split(origins, ",") {
		if s := strings.TrimSpace(o); s != "" {
			cfg.CORSAllowedOrigins = append(cfg.CORSAllowedOrigins, s)
		}
	}

	// JWT secret 必须与 sub2api 完全一致，否则无法验证其签发的 token。
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required (must match sub2api)")
	}
	if cfg.DB.Password == "" {
		return nil, fmt.Errorf("DATABASE_PASSWORD is required")
	}

	return cfg, nil
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// loadDotEnv 把 .env 里的键值注入进程环境。已存在的环境变量优先（不覆盖），
// 找不到文件时静默跳过——生产环境直接用真实环境变量即可。
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}
