package gitlab

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const (
	tokenName = "GITLAB_TOKEN"
	gitlabURL = "GITLAB_URL"
)

func TestUserInfo(t *testing.T) {
	logger := zaptest.NewLogger(t)
	token := os.Getenv(tokenName)
	url := os.Getenv(gitlabURL)

	if token == "" || url == "" {
		t.Skip("skipping integration test. Add GITLAB_TOKEN and GITLAB_HOST in order to run")
	}

	client := NewClient(logger, token, url, PrivateTokenType)
	ctx := context.Background()

	userInfo, err := client.GetUserInfo(ctx)
	require.Nil(t, err)

	require.NotNil(t, userInfo)
	require.NotEqual(t, "", userInfo.ID)
}
