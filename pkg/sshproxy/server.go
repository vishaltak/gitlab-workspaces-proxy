package sshproxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"gitlab.com/remote-development/gitlab-workspaces-proxy/internal/logz"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/config"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/gitlab"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/upstream"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

type workspaceNameCtxValueType string

const (
	workspaceNameCtxValueKey workspaceNameCtxValueType = "workspace_name"

	MaxAuthTries = 3
)

var errUserNotAllowedAccessToWorkspace = errors.New("user not allowed access to workspace")

type SSHProxy struct {
	tracker         *upstream.Tracker
	log             *zap.Logger
	sshConfig       *config.SSH
	commonSSHConfig *ssh.ServerConfig
}

func New(ctx context.Context, logger *zap.Logger, tracker *upstream.Tracker, sshConfig *config.SSH, apiFactory gitlab.APIFactory) (*SSHProxy, error) {
	hostKeySigner, parseErr := ssh.ParsePrivateKey([]byte(sshConfig.HostKey))
	if parseErr != nil {
		logger.Error("failed to read host key", logz.Error(parseErr), logz.SSHHostKey(sshConfig.HostKey))
		return nil, parseErr
	}

	serverConfig := &ssh.ServerConfig{
		MaxAuthTries: MaxAuthTries,
		PasswordCallback: func(c ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			// Validate password using API call
			callbackCtx, cancel := context.WithTimeout(ctx, time.Second*60)
			defer cancel()

			// We are using the workspace name as the username so that we can identify the correct workspace
			// so that we can authenticate and authorize the user. We looked into other options such as passing
			// options in the SSH command however that would not be available during the auth stage of the
			// connection.
			workspaceName := c.User()
			err := validateWorkspaceOwnership(callbackCtx, workspaceName, string(password), tracker, apiFactory)
			if err != nil {
				logger.Error("failed to validate ownership of workspace",
					logz.Error(err),
					logz.WorkspaceName(workspaceName),
				)
				return nil, err
			}

			return &ssh.Permissions{
				Extensions: map[string]string{
					"workspaceName": c.User(),
				},
			}, nil
		},
	}

	serverConfig.AddHostKey(hostKeySigner)

	return &SSHProxy{
		tracker:         tracker,
		log:             logger,
		sshConfig:       sshConfig,
		commonSSHConfig: serverConfig,
	}, nil
}

// handleSSHConnection should always be called in a goroutine.
func (p *SSHProxy) handleSSHConnection(ctx context.Context, incomingConn net.Conn) {
	clientConn, clientChannel, clientReqChannel, err := ssh.NewServerConn(incomingConn, p.commonSSHConfig)
	if err != nil {
		p.log.Error("failed to create SSH connection", logz.Error(err))
		return
	}

	if clientConn.Permissions == nil || clientConn.Permissions.Extensions == nil || clientConn.Permissions.Extensions["workspaceName"] == "" {
		// TODO: log all fields - e.g. remote addr, permissions.extensions, etc.
		p.log.Error("failed to find workspace name in connection-permission-extension")
		p.closeConnection(incomingConn, "indeterminable")
		return
	}

	workspaceName := clientConn.Permissions.Extensions["workspaceName"]
	connCtx, connCancel := context.WithCancel(ctx)
	connCtx = context.WithValue(connCtx, workspaceNameCtxValueKey, workspaceName)
	defer connCancel()
	defer p.closeConnection(clientConn, workspaceName)

	upstreamHostMapping, err := p.tracker.GetByWorkspaceName(workspaceName)
	if err != nil {
		p.log.Error("failed to find workspace name in tracker", logz.Error(err))
		return
	}

	remoteAddr := fmt.Sprintf("%s:%d", upstreamHostMapping.Backend, p.sshConfig.BackendPort)
	remoteConn, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		p.log.Error("failed to create backend connection", logz.Error(err), logz.WorkspaceName(workspaceName))
		return
	}
	// before use, a handshake must be performed on the incoming net.Conn.
	backendConn, backendChannel, backendReqChannel, err := ssh.NewClientConn(remoteConn, remoteAddr, &ssh.ClientConfig{
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		User:            p.sshConfig.BackendUsername,
		Auth: []ssh.AuthMethod{
			ssh.Password(""),
		},
	})
	if err != nil {
		p.log.Error("failed to create backend connection handshake",
			logz.Error(err),
			logz.WorkspaceName(workspaceName),
		)
		return
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		p.copyRequests(connCtx, clientReqChannel, backendConn)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		p.copyRequests(connCtx, backendReqChannel, clientConn)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		p.copyData(connCtx, clientConn, backendChannel)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		p.copyData(connCtx, backendConn, clientChannel)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		waitErr := clientConn.Wait()
		if waitErr != nil {
			p.log.Error("failed to wait for client connection", logz.Error(waitErr), logz.WorkspaceName(workspaceName))
			connCancel()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		waitErr := backendConn.Wait()
		if waitErr != nil {
			p.log.Error("failed to wait for backend connection", logz.Error(waitErr), logz.WorkspaceName(workspaceName))
			connCancel()
		}
	}()

	wg.Wait()
}

func (p *SSHProxy) Start(ctx context.Context, listenAddr string, readyCh chan<- struct{}, stopCh chan<- struct{}) error {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		p.log.Error("failed to start ssh proxy server.", logz.Error(err))
		return fmt.Errorf("failed to start ssh proxy server: %v", err)
	}

	go func() {
		<-ctx.Done()
		closeErr := listener.Close()
		if closeErr != nil {
			p.log.Error("failed to close listener for ssh proxy", logz.Error(closeErr))
		}
	}()

	if readyCh != nil {
		readyCh <- struct{}{}
	}

	for {
		incomingConn, err := listener.Accept()
		if err != nil {
			// We need to detect if the connection was closed due to the context being cancelled, if so
			// then we shouldn't continue the loop
			if errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "use of closed network connection") {
				break
			}
			// TODO: log the field of the connection - e.g. remote addr, local addr, etc.
			p.log.Error("failed to accept incoming connection", logz.Error(err))
			continue
		}

		go p.handleSSHConnection(ctx, incomingConn)
	}

	if stopCh != nil {
		stopCh <- struct{}{}
	}

	return nil
}

func validateWorkspaceOwnership(ctx context.Context, workspaceName, password string, tracker *upstream.Tracker, apiFactory gitlab.APIFactory) error {
	api := apiFactory(password)

	user, err := api.GetUserInfo(ctx)
	if err != nil {
		return err
	}

	upstreamHostMapping, err := tracker.GetByWorkspaceName(workspaceName)
	if err != nil {
		return err
	}

	workspace, err := api.GetWorkspace(ctx, upstreamHostMapping.WorkspaceID)
	if err != nil {
		return err
	}

	if workspace.User.ID != user.ID {
		// TODO: log which user was trying to access this workspace
		return errUserNotAllowedAccessToWorkspace
	}

	return nil
}

type connection interface {
	Close() error
}

func (p *SSHProxy) closeConnection(conn connection, workspaceName string) {
	err := conn.Close()
	if err != nil {
		p.log.Error(
			"failed to close connection",
			logz.Error(err),
			logz.WorkspaceName(workspaceName),
		)
	}
}
