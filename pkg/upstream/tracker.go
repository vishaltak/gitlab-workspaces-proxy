package upstream

import (
	"errors"
	"sync"

	"gitlab.com/remote-development/gitlab-workspaces-proxy/internal/logz"
	"go.uber.org/zap"
)

type HostMapping struct {
	Hostname        string `yaml:"host"`
	BackendPort     int32  `yaml:"port"`
	Backend         string `yaml:"backend"`
	BackendProtocol string `yaml:"protocol"`
	WorkspaceID     string `yaml:"workspaceID"`
	WorkspaceName   string `yaml:"workspaceName"`
}

type Tracker struct {
	logger          *zap.Logger
	upstreamsByHost map[string]HostMapping
	upstreamsByName map[string]HostMapping
	sync.RWMutex
}

func NewTracker(logger *zap.Logger) *Tracker {
	return &Tracker{
		logger:          logger,
		upstreamsByHost: make(map[string]HostMapping),
		upstreamsByName: make(map[string]HostMapping),
	}
}

var ErrNotFound = errors.New("host mapping not found")

func (u *Tracker) GetByHostname(name string) (*HostMapping, error) {
	u.RLock()
	defer u.RUnlock()
	if val, ok := u.upstreamsByHost[name]; ok {
		return &val, nil
	}

	return nil, ErrNotFound
}

func (u *Tracker) GetByWorkspaceName(name string) (*HostMapping, error) {
	u.RLock()
	defer u.RUnlock()
	if val, ok := u.upstreamsByName[name]; ok {
		return &val, nil
	}

	return nil, ErrNotFound
}

func (u *Tracker) Add(mapping HostMapping) {
	u.Lock()
	defer u.Unlock()
	u.upstreamsByHost[mapping.Hostname] = mapping
	u.upstreamsByName[mapping.WorkspaceName] = mapping
	u.logger.Info("host mapping added",
		logz.HostMappingHostname(mapping.Hostname),
		logz.HostMappingBackend(mapping.Backend),
		logz.HostMappingBackendPort(mapping.BackendPort),
		logz.HostMappingBackendProtocol(mapping.BackendProtocol),
		logz.WorkspaceName(mapping.WorkspaceName),
	)
}

func (u *Tracker) DeleteByHostname(name string) {
	u.Lock()
	defer u.Unlock()
	mapping := u.upstreamsByHost[name]
	workspaceName := mapping.WorkspaceName
	delete(u.upstreamsByName, workspaceName)
	delete(u.upstreamsByHost, name)
	u.logger.Info("host mapping removed",
		logz.HostMappingHostname(mapping.Hostname),
		logz.HostMappingBackend(mapping.Backend),
		logz.HostMappingBackendPort(mapping.BackendPort),
		logz.HostMappingBackendProtocol(mapping.BackendProtocol),
		logz.WorkspaceName(mapping.WorkspaceName),
	)
}
