package upstream

import (
	"errors"
	"sync"

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

var ErrNotFound = errors.New("upstream not found")

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
	u.logger.Info("New upstream added", zap.String("host", mapping.Hostname), zap.String("backend", mapping.Backend), zap.Int32("backend_port", mapping.BackendPort))
}

func (u *Tracker) DeleteByHostname(name string) {
	u.Lock()
	defer u.Unlock()
	workspaceName := u.upstreamsByHost[name].WorkspaceName
	delete(u.upstreamsByName, workspaceName)
	delete(u.upstreamsByHost, name)
	u.logger.Info("Upstream removed", zap.String("host", name))
}
