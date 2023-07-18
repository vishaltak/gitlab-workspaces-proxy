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
	LogLevel    string      `yaml:"log_level"`
	HTTP        HTTP        `yaml:"http"`
	SSH         SSH         `yaml:"ssh"`
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

	if c.LogLevel == "" {
		c.LogLevel = "info"
	}

	c.setHTTPDefaults()
	c.setSSHDefaults()
	return nil
}

func (c *Config) setSSHDefaults() {
	if c.SSH.BackendUsername == "" {
		c.SSH.BackendUsername = "gitlab-workspaces"
	}

	if c.SSH.BackendPort == 0 {
		c.SSH.BackendPort = 22
	}
}

func (c *Config) setHTTPDefaults() {
	if c.HTTP.Port == 0 {
		c.HTTP.Port = 9876
	}
}

func (c *Config) GetZapLevel() (zap.AtomicLevel, error) {
	var zapLevel zap.AtomicLevel
	err := zapLevel.UnmarshalText([]byte(c.LogLevel))
	if err != nil {
		return zap.NewAtomicLevel(), err
	}
	return zapLevel, nil
}
