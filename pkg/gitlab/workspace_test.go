package gitlab

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetWorkspace(t *testing.T) {
	token := os.Getenv(tokenName)
	url := os.Getenv(gitlabURL)

	if token == "" || url == "" {
		t.Skip("skipping integration test. Add GITLAB_TOKEN and GITLAB_HOST in order to run")
	}

	client := NewClient(token, url, PrivateTokenType)
	ctx := context.Background()

	workspace, err := client.GetWorkspace(ctx, "1")
	require.Nil(t, err)

	require.NotNil(t, workspace)
	require.NotEqual(t, "", workspace.User.Username)
}
