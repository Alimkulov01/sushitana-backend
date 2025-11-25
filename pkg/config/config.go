package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/fx"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

var Module = fx.Provide(NewConfig)

type IConfig interface {
	Get(key string) interface{}
	GetBool(key string) bool
	GetFloat64(key string) float64
	GetInt(key string) int
	GetInt64(key string) int64
	GetIntSlice(key string) []int
	GetString(key string) string
	GetStringMap(key string) map[string]interface{}
	GetStringMapString(key string) map[string]string
	UnmarshalKey(key string, val interface{}) error
	GetStringSlice(key string) []string
	GetDuration(key string) time.Duration
}

type config struct {
	cfg *viper.Viper
}

func NewConfig() IConfig {
	_ = godotenv.Load()

	cfg := viper.New()
	cfg.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	cfg.AutomaticEnv()

	_ = cfg.BindEnv("server.host", "SERVICE_HOST")
	_ = cfg.BindEnv("server.port", "SERVICE_HTTP_PORT")
	_ = cfg.BindEnv("database.dns", "DATABASE_DNS")
	_ = cfg.BindEnv("database.migration", "DATABASE_MIGRATION")
	_ = cfg.BindEnv("database.host", "POSTGRES_HOST")
	_ = cfg.BindEnv("database.user", "POSTGRES_USER")
	_ = cfg.BindEnv("database.password", "POSTGRES_PASSWORD")
	_ = cfg.BindEnv("database.dbname", "POSTGRES_DATABASE")
	_ = cfg.BindEnv("database.port", "POSTGRES_PORT")
	_ = cfg.BindEnv("database.pool_max_conns", "POSTGRES_MAX_CONNECTION")
	_ = cfg.BindEnv("database.pool_max_conn_lifetime", "POSTGRES_POOL_MAX_CONN_LIFETIME")
	_ = cfg.BindEnv("aws_access_key_id", "AWS_ACCESS_KEY_ID")
	_ = cfg.BindEnv("aws_secret_access_key", "AWS_SECRET_ACCESS_KEY")
	_ = cfg.BindEnv("aws_region", "AWS_REGION")
	_ = cfg.BindEnv("aws_s3_bucket", "AWS_S3_BUCKET")
	_ = cfg.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = cfg.BindEnv("redis.addrs", "REDIS_ADDRS")
	_ = cfg.BindEnv("admin_chat_id", "ADMIN_CHAT_ID")
	_ = cfg.BindEnv("gin.trusted_proxies", "GIN_TRUSTED_PROXIES")
	_ = cfg.BindEnv("bot_token_sushitana", "BOT_TOKEN_SUSHITANA")

	if addrs := os.Getenv("REDIS_ADDRS"); addrs != "" {
		cfg.Set("redis.addrs", strings.Split(addrs, ","))
	}

	if cfg.GetString("database.dns") == "" {
		if dsn := BuildPostgresDSNFromViper(cfg); dsn != "" {
			cfg.Set("database.dns", dsn)
		}
	}
	if cfg.GetString("database.url") == "" {
		if url := BuildPostgresURLFromViper(cfg); url != "" {
			cfg.Set("database.url", url)
		}
	}
	if cfg.GetString("database.migration") == "" {
		if url := BuildPostgresURLFromViper(cfg); url != "" {
			cfg.Set("database.migration", url)
		}
	}

	return &config{cfg: cfg}
}

func (c *config) Get(key string) interface{} {
	return c.cfg.Get(key)
}

func (c *config) GetBool(key string) bool {
	return c.cfg.GetBool(key)
}

func (c *config) GetFloat64(key string) float64 {
	return c.cfg.GetFloat64(key)
}

func (c *config) GetInt(key string) int {
	return c.cfg.GetInt(key)
}

func (c *config) GetInt64(key string) int64 {
	return c.cfg.GetInt64(key)
}

func (c *config) GetIntSlice(key string) []int {
	return c.cfg.GetIntSlice(key)
}

func (c *config) GetString(key string) string {
	return c.cfg.GetString(key)
}

func (c *config) GetStringSlice(key string) []string {
	return c.cfg.GetStringSlice(key)
}

func (c *config) GetStringMap(key string) map[string]interface{} {
	return c.cfg.GetStringMap(key)
}
func (c *config) GetStringMapString(key string) map[string]string {
	return c.cfg.GetStringMapString(key)
}

func (c *config) UnmarshalKey(key string, val interface{}) error {
	return c.cfg.UnmarshalKey(key, &val)
}

func (c *config) GetDuration(key string) time.Duration {
	return c.cfg.GetDuration(key)
}

func BuildPostgresDSNFromViper(v *viper.Viper) string {
	user := v.GetString("database.user")
	if user == "" {
		user = v.GetString("POSTGRES_USER")
	}
	password := v.GetString("database.password")
	if password == "" {
		password = v.GetString("POSTGRES_PASSWORD")
	}
	dbname := v.GetString("database.dbname")
	if dbname == "" {
		dbname = v.GetString("POSTGRES_DATABASE")
	}
	host := v.GetString("database.host")
	if host == "" {
		host = v.GetString("POSTGRES_HOST")
	}
	port := v.GetString("database.port")
	if port == "" {
		port = v.GetString("POSTGRES_PORT")
	}
	poolMaxConns := v.GetInt("database.pool_max_conns")
	if poolMaxConns == 0 {
		poolMaxConns = v.GetInt("POSTGRES_MAX_CONNECTION")
	}
	if poolMaxConns == 0 {
		poolMaxConns = 30
	}
	poolLifetime := v.GetString("database.pool_max_conn_lifetime")
	if poolLifetime == "" {
		poolLifetime = "1h30m"
	}

	if user == "" && host == "" && dbname == "" {
		return ""
	}

	parts := []string{}
	if user != "" {
		parts = append(parts, "user="+user)
	}
	if password != "" {
		parts = append(parts, "password="+password)
	}
	if dbname != "" {
		parts = append(parts, "dbname="+dbname)
	}
	if host != "" {
		parts = append(parts, "host="+host)
	}
	if port != "" {
		parts = append(parts, "port="+port)
	}
	parts = append(parts, fmt.Sprintf("pool_max_conns=%d", poolMaxConns))
	parts = append(parts, fmt.Sprintf("pool_max_conn_lifetime=%s", poolLifetime))

	return strings.Join(parts, " ")
}

func BuildPostgresURLFromViper(v *viper.Viper) string {
	user := v.GetString("database.user")
	if user == "" {
		user = v.GetString("POSTGRES_USER")
	}
	password := v.GetString("database.password")
	if password == "" {
		password = v.GetString("POSTGRES_PASSWORD")
	}
	host := v.GetString("database.host")
	if host == "" {
		host = v.GetString("POSTGRES_HOST")
	}
	port := v.GetString("database.port")
	if port == "" {
		port = v.GetString("POSTGRES_PORT")
	}
	dbname := v.GetString("database.dbname")
	if dbname == "" {
		dbname = v.GetString("POSTGRES_DATABASE")
	}

	if user == "" || host == "" || dbname == "" {
		return ""
	}

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user,
		password,
		host,
		port,
		dbname,
	)
}
