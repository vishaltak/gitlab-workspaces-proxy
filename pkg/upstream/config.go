package upstream

type HostMapping struct {
	Host            string `yaml:"host"`
	BackendPort     int32  `yaml:"port"`
	Backend         string `yaml:"backend"`
	BackendProtocol string `yaml:"protocol"`
}
