package upstream

type HostMapping struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	Backend         string `yaml:"backend"`
	BackendProtocol string `yaml:"protocol"`
}