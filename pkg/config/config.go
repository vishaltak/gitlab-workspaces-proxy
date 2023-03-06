package config

import (
	"os"

	"gitlab.com/remote-development/auth-proxy/pkg/auth"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Auth    auth.AuthConfig `yaml:"auth"`
	BaseUrl string          `yaml:"base_url"`
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

	return &result, nil
}
