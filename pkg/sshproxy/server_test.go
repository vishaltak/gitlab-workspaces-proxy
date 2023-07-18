package sshproxy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/gitlab"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/upstream"
	"go.uber.org/zap/zaptest"
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
