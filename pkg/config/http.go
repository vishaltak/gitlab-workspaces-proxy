package config

type HTTP struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}
