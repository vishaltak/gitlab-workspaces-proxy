package gitlab

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	tokenName = "GITLAB_TOKEN"
	gitlabURL = "GITLAB_URL"
)

func TestUserInfo(t *testing.T) {
	token := os.Getenv(tokenName)
	url := os.Getenv(gitlabURL)

	if token == "" || url == "" {
		t.Skip("skipping integration test. Add GITLAB_TOKEN and GITLAB_HOST in order to run")
	}

	client := NewClient(token, url, PrivateTokenType)
	ctx := context.Background()

	userInfo, err := client.GetUserInfo(ctx)
	require.Nil(t, err)

	require.NotNil(t, userInfo)
	require.NotEqual(t, "", userInfo.ID)
}
