// Package config loads runtime configuration from a YAML file using
// Viper. Environment variables override individual keys: upper-case the key
// and replace "." with "_" (jwt.secret -> JWT_SECRET).
package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	Log    LogConfig    `mapstructure:"log"`
	Mongo  MongoConfig  `mapstructure:"mongo"`
	JWT    JWTConfig    `mapstructure:"jwt"`
}

type ServerConfig struct {
	HTTPPort       int  `mapstructure:"http_port"`
	GRPCPort       int  `mapstructure:"grpc_port"`
	GRPCReflection bool `mapstructure:"grpc_reflection"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
}

type MongoConfig struct {
	URI      string `mapstructure:"uri"`
	Database string `mapstructure:"database"`
}

type JWTConfig struct {
	Secret string        `mapstructure:"secret"` // HMAC key for HS256; required
	Issuer string        `mapstructure:"issuer"`
	TTL    time.Duration `mapstructure:"ttl"`
}

// defaults apply to keys missing from the file. Every key must be listed
// here (or in the file) for its environment override to be picked up.
var defaults = map[string]any{
	"server.http_port":       8080,
	"server.grpc_port":       50051,
	"server.grpc_reflection": true,
	"log.level":              "info",
	"mongo.uri":              "mongodb://localhost:27017",
	"mongo.database":         "user_management",
	"jwt.secret":             "",
	"jwt.issuer":             "user-management-api",
	"jwt.ttl":                "1h",
}

// InitialConfig reads the YAML file at path, applies environment
// overrides, and validates the result.
func InitialConfig(path string) (*Config, error) {
	v := viper.New()
	for k, val := range defaults {
		v.SetDefault(k, val)
	}

	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %q: %w (copy config/config.example.yaml to get started)", path, err)
	}

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config %q: %w", path, err)
	}
	return &cfg, nil
}

// Validate reports every problem at once rather than failing on the first.
func (c Config) Validate() error {
	var errs []error
	if c.JWT.Secret == "" {
		errs = append(errs, errors.New("jwt.secret is required (generate one with: openssl rand -base64 48)"))
	}
	if c.JWT.TTL <= 0 {
		errs = append(errs, errors.New("jwt.ttl must be positive"))
	}
	if c.Mongo.URI == "" {
		errs = append(errs, errors.New("mongo.uri is required"))
	}
	if c.Mongo.Database == "" {
		errs = append(errs, errors.New("mongo.database is required"))
	}
	for name, p := range map[string]int{"server.http_port": c.Server.HTTPPort, "server.grpc_port": c.Server.GRPCPort} {
		if p < 1 || p > 65535 {
			errs = append(errs, fmt.Errorf("%s must be 1-65535, got %d", name, p))
		}
	}
	return errors.Join(errs...)
}
