package config

type SSH struct {
	Enabled         bool   `yaml:"enabled"`
	Port            int    `yaml:"port"`
	HostKey         string `yaml:"host_key"`
	BackendPort     int    `yaml:"backend_port"`
	BackendUsername string `yaml:"backend_username"`
}
