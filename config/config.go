package config

import (
	"os"

	"gitlab.com/remote-development/auth-proxy/auth"
	"gitlab.com/remote-development/auth-proxy/upstream"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Auth      auth.AuthConfig        `yaml:"auth"`
	Upstreams []upstream.HostMapping `yaml:"upstreams"`
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
