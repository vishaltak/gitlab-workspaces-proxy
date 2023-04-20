package upstream

import (
	"errors"
	"sync"

	"go.uber.org/zap"
)

type HostMapping struct {
	Host            string `yaml:"host"`
	BackendPort     int32  `yaml:"port"`
	Backend         string `yaml:"backend"`
	BackendProtocol string `yaml:"protocol"`
	WorkspaceID     string `yaml:"workspaceID"`
}

type Tracker struct {
	logger    *zap.Logger
	upstreams map[string]HostMapping
	sync.RWMutex
}

func NewTracker(logger *zap.Logger) *Tracker {
	return &Tracker{
		logger:    logger,
		upstreams: make(map[string]HostMapping),
	}
}

var ErrNotFound = errors.New("upstream not found")

func (u *Tracker) Get(name string) (*HostMapping, error) {
	u.RLock()
	defer u.RUnlock()
	if val, ok := u.upstreams[name]; ok {
		return &val, nil
	}

	return nil, ErrNotFound
}

func (u *Tracker) Add(mapping HostMapping) {
	u.Lock()
	defer u.Unlock()
	u.upstreams[mapping.Host] = mapping
	u.logger.Info("New upstream added", zap.String("host", mapping.Host), zap.String("backend", mapping.Backend), zap.Int32("backend_port", mapping.BackendPort))
}

func (u *Tracker) Delete(host string) {
	u.Lock()
	defer u.Unlock()
	delete(u.upstreams, host)
	u.logger.Info("Upstream removed", zap.String("host", host))
}
