package config

import (
	"log/slog"
	"net"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	DynamoDBLocalAddr string `env:"DYNAMODB_LOCAL_ADDR" envDefault:"localhost:8000"`

	Host string `env:"HOST" envDefault:"localhost"`
	Port string `env:"PORT" envDefault:"8888"`

	LogLevel slog.Level `env:"LOG_LEVEL" envDefault:"INFO"`
}

func Parse() (*Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) BindAddr() string {
	return net.JoinHostPort(c.Host, c.Port)
}
