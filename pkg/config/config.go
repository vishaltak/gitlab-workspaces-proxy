package config

import (
	"errors"
	"os"

	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/auth"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

var errAuthConfigInvalid = errors.New("auth config invalid")

type Config struct {
	Auth        auth.Config `yaml:"auth"`
	MetricsPath string      `yaml:"metrics_path"`
	Port        int         `yaml:"port"`
	LogLevel    string      `yaml:"log_level"`
}

func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var result Config

	err = yaml.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}

	err = result.setDefaults()
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Config) setDefaults() error {
	if c.Auth.ClientID == "" || c.Auth.ClientSecret == "" || c.Auth.Host == "" || c.Auth.RedirectURI == "" || c.Auth.SigningKey == "" {
		return errAuthConfigInvalid
	}

	if c.MetricsPath == "" {
		c.MetricsPath = "/metrics"
	}

	if c.Port == 0 {
		c.Port = 9876
	}

	if c.LogLevel == "" {
		c.LogLevel = "info"
	}

	return nil
}

func (c *Config) GetZapLevel() (zap.AtomicLevel, error) {
	var zapLevel zap.AtomicLevel
	err := zapLevel.UnmarshalText([]byte(c.LogLevel))
	if err != nil {
		return zap.NewAtomicLevel(), err
	}
	return zapLevel, nil
}
