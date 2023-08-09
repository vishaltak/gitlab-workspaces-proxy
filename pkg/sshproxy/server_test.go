package sshproxy

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/config"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/gitlab"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/upstream"
	"go.uber.org/zap/zaptest"
	"golang.org/x/crypto/ssh"
)

func TestValidateWorkspaceOwnership(t *testing.T) {
	tests := []struct {
		description      string
		workspaceName    string
		password         string
		userID           int
		workspaceOwnerID int
		expectError      bool
	}{
		{
			description:      "When workspace exists and password is valid should not throw an error",
			workspaceName:    "test",
			password:         "password",
			userID:           1,
			workspaceOwnerID: 1,
			expectError:      false,
		},
		{
			description:      "When invalid password is passed should throw error",
			workspaceName:    "test",
			password:         "pass",
			userID:           1,
			workspaceOwnerID: 1,
			expectError:      true,
		},
		{
			description:      "When workspace does not exist should throw error",
			workspaceName:    "wrong_name",
			password:         "password",
			userID:           1,
			workspaceOwnerID: 1,
			expectError:      true,
		},
		{
			description:      "When user is not workspace owner should throw error",
			workspaceName:    "test",
			password:         "password",
			userID:           1,
			workspaceOwnerID: 2,
			expectError:      true,
		},
	}

	logger := zaptest.NewLogger(t)
	tracker := upstream.NewTracker(logger)
	tracker.Add(upstream.HostMapping{
		Hostname:        "test.workspaces.gitlab.com",
		BackendPort:     60022,
		Backend:         "test.mynamespace",
		BackendProtocol: "https",
		WorkspaceName:   "test",
		WorkspaceID:     "1",
	})

	ctx := context.Background()

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			apiFactory := func(token string) gitlab.API {
				return &gitlab.MockAPI{
					GetUserInfoUserID:  tc.userID,
					GetWorkspaceUserID: tc.workspaceOwnerID,
					ValidToken:         "password",
					AccessToken:        token,
				}
			}

			err := validateWorkspaceOwnership(ctx, tc.workspaceName, tc.password, tracker, apiFactory)
			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestServerStartsAndExits(t *testing.T) {
	port := 30010
	addr := fmt.Sprintf(":%d", port)
	logger := zaptest.NewLogger(t)
	tracker := upstream.NewTracker(logger)
	hostKey, err := os.ReadFile("./fixtures/ssh-host-key")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server, err := New(ctx, logger, tracker, &config.SSH{
		HostKey: string(hostKey),
	}, gitlab.MockAPIFactory)
	require.NoError(t, err)

	readyCh := make(chan struct{})

	go func() {
		err = server.Start(ctx, addr, readyCh, nil)
		require.NoError(t, err)
	}()

	<-readyCh

	time.Sleep(1 * time.Second)
	cancel()
	time.Sleep(1 * time.Second)

	// Verify that we can't connect to the server
	serverAddr := fmt.Sprintf("localhost:%d", port)
	_, err = net.Dial("tcp", serverAddr)
	require.Error(t, err)
}

func TestServerAuth(t *testing.T) {
	logger := zaptest.NewLogger(t)
	hostKey, err := os.ReadFile("./fixtures/ssh-host-key")
	require.NoError(t, err)

	tt := []struct {
		description   string
		port          int
		userID        int
		upstream      *upstream.HostMapping
		workspaceName string
		expectError   bool
	}{
		{
			description: "Server does not accept connections when no upstreams are found",
			port:        30011,
			userID:      1,
			upstream: &upstream.HostMapping{
				Hostname:      "myworkspace1",
				WorkspaceID:   "myworkspace1",
				WorkspaceName: "myworkspace1",
			},
			workspaceName: "test",
			expectError:   true,
		},
		{
			description: "Server does not accept connections when upstream found but PAT not validated",
			port:        30012,
			userID:      2,
			upstream: &upstream.HostMapping{
				Hostname:      "myworkspace",
				WorkspaceID:   "myworkspace",
				WorkspaceName: "myworkspace",
			},
			workspaceName: "myworkspace",
			expectError:   true,
		},
		{
			description: "Server does accept connections when upstream found and PAT is correct",
			port:        30013,
			userID:      1,
			upstream: &upstream.HostMapping{
				Hostname:      "myworkspace",
				WorkspaceID:   "myworkspace",
				WorkspaceName: "myworkspace",
			},
			workspaceName: "myworkspace",
			expectError:   false,
		},
	}

	for _, test := range tt {
		t.Run(test.description, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tracker := upstream.NewTracker(logger)
			if test.upstream != nil {
				tracker.Add(*test.upstream)
			}

			server, err := New(ctx, logger, tracker, &config.SSH{
				HostKey: string(hostKey),
			}, createFactory(test.userID, 1))
			require.NoError(t, err)

			addr := fmt.Sprintf(":%d", test.port)
			readyCh := make(chan struct{})
			stopCh := make(chan struct{})

			go func(addr string) {
				err = server.Start(ctx, addr, readyCh, stopCh)
				require.NoError(t, err)
			}(addr)

			<-readyCh

			serverAddr := fmt.Sprintf("localhost%s", addr)
			serverConn, err := net.Dial("tcp", serverAddr)
			require.NoError(t, err)
			defer func() {
				serverConn.Close()
				cancel()
				<-stopCh
				_ = logger.Sync()
				// Wait for all logs to be written
				time.Sleep(1 * time.Second)
			}()

			_, _, _, err = ssh.NewClientConn(serverConn, serverAddr, &ssh.ClientConfig{
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
				User:            test.workspaceName,
				Auth: []ssh.AuthMethod{
					ssh.Password(""),
				},
			})
			if test.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func createFactory(userID, workspaceUserID int) gitlab.APIFactory {
	return func(token string) gitlab.API {
		return &gitlab.MockAPI{
			GetUserInfoUserID:  userID,
			GetWorkspaceUserID: workspaceUserID,
			ValidToken:         token,
			AccessToken:        token,
		}
	}
}
