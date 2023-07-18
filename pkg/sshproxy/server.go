package sshproxy

import (
	"context"
	"fmt"
	"net"
	"time"

	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/config"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/gitlab"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/upstream"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

const (
	MaxAuthTries = 3
)

type sshProxy struct {
	tracker         *upstream.Tracker
	log             *zap.Logger
	sshConfig       *config.SSH
	commonSSHConfig *ssh.ServerConfig
}

func New(logger *zap.Logger, tracker *upstream.Tracker, sshConfig *config.SSH, apiFactory gitlab.APIFactory) (*sshProxy, error) {
	hostKeySigner, err := ssh.ParsePrivateKey([]byte(sshConfig.HostKey))
	if err != nil {
		logger.Error("Error reading host key", zap.Error(err))
		return nil, err
	}

	serverConfig := &ssh.ServerConfig{
		MaxAuthTries: MaxAuthTries,
		PasswordCallback: func(c ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			// Validate password using API call
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
			defer cancel()

			// We are using the workspace name as the user name so that we can identify the correct workspace
			// so that we can authenticate and authorize the user. We looked into other options such as passing
			// options in the SSH command however that would not be available during the auth stage of the
			// connection.
			workspaceName := c.User()
			err := validateWorkspaceOwnership(ctx, workspaceName, string(password), tracker, apiFactory)
			if err != nil {
				logger.Error("unable to validate ownership of the workspace", zap.Error(err))
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

	return &sshProxy{
		tracker:         tracker,
		log:             logger,
		sshConfig:       sshConfig,
		commonSSHConfig: serverConfig,
	}, nil
}

func (p *sshProxy) handleSSHConnection(incomingConn net.Conn) {
	defer p.closeConnectionAndLogError(incomingConn, "error closing incoming connection", "unknown")

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	clientConn, clientChannel, clientReqChannel, err := ssh.NewServerConn(incomingConn, p.commonSSHConfig)
	if err != nil {
		p.log.Error("failed to create SSH connection", zap.Error(err))
		return
	}

	if clientConn.Permissions == nil || clientConn.Permissions.Extensions == nil || clientConn.Permissions.Extensions["workspaceName"] == "" {
		p.log.Error("could not find workspace name in permissions", zap.Error(err))
		return
	}

	workspaceName := clientConn.Permissions.Extensions["workspaceName"]
	defer p.closeConnectionAndLogError(clientConn, "failed to close client connection", workspaceName)

	upstream, err := p.tracker.GetByWorkspaceName(workspaceName)
	if err != nil {
		p.log.Error("could not find workspace name in tracker", zap.Error(err))
		return
	}

	remoteAddr := fmt.Sprintf("%s:%d", upstream.Backend, p.sshConfig.BackendPort)
	remoteConn, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		p.log.Error("Error creating backend connection", zap.Error(err))
		return
	}
	defer p.closeConnectionAndLogError(remoteConn, "could not close remote connection", workspaceName)

	backendConn, backendChannel, backendReqChannel, err := ssh.NewClientConn(remoteConn, remoteAddr, &ssh.ClientConfig{
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		User:            p.sshConfig.BackendUsername,
		Auth: []ssh.AuthMethod{
			ssh.Password(""),
		},
	})
	if err != nil {
		p.log.Error("error creating backend connection", zap.Error(err))
		return
	}

	go p.copyRequests(clientReqChannel, backendConn)
	go p.copyRequests(backendReqChannel, clientConn)
	go p.copyData(clientConn, backendChannel)
	go p.copyData(backendConn, clientChannel)

	go func() {
		err = clientConn.Wait()
		if err != nil {
			p.log.Error("waiting for client connection failed", zap.Error(err))
		}
		cancel()
	}()
	go func() {
		err = backendConn.Wait()
		if err != nil {
			p.log.Error("error waiting for backend connection", zap.Error(err))
		}
		cancel()
	}()

	<-ctx.Done()
}

func (p *sshProxy) Start(listenAddr string) error {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		p.log.Error("failed to start proxy server.", zap.Error(err))
		return fmt.Errorf("failed to start proxy server: %v", err)
	}

	for {
		incomingConn, err := listener.Accept()
		if err != nil {
			p.log.Error("Failed to accept incoming connection: %v", zap.Error(err))
			continue
		}

		go p.handleSSHConnection(incomingConn)
	}
}

func validateWorkspaceOwnership(ctx context.Context, workspaceName, password string, tracker *upstream.Tracker, apiFactory gitlab.APIFactory) error {
	api := apiFactory(password)

	user, err := api.GetUserInfo(ctx)
	if err != nil {
		return err
	}

	backend, err := tracker.GetByWorkspaceName(workspaceName)
	if err != nil {
		return err
	}

	workspace, err := api.GetWorkspace(ctx, backend.WorkspaceID)
	if err != nil {
		return err
	}

	if workspace.User.ID != user.ID {
		return fmt.Errorf("unauthorized for workspace %s", backend.WorkspaceName)
	}

	return nil
}

type connection interface {
	Close() error
}

func (p *sshProxy) closeConnectionAndLogError(conn connection, msg, workspaceName string) {
	err := conn.Close()
	if err != nil {
		p.log.Error(msg, zap.String("workspace_name", workspaceName), zap.Error(err))
	}
}
