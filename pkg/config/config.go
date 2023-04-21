package config

import (
	"errors"
	"os"

	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/auth"
	"gopkg.in/yaml.v3"
)

var errAuthConfigInvalid = errors.New("auth config invalid")

type Config struct {
	Auth        auth.Config `yaml:"auth"`
	MetricsPath string      `yaml:"metrics_path"`
	Port        int         `yaml:"port"`
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

	return nil
}
