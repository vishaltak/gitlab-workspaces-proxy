package auth

import (
	"context"
	"errors"

	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/gitlab"
)

var ErrInvalidUser = errors.New("user does not have access to this workspace")

func checkAuthorization(ctx context.Context, accessToken string, workspaceID string, apiFactory gitlab.APIFactory) error {
	api := apiFactory(accessToken)

	currentUser, err := api.GetUserInfo(ctx)
	if err != nil {
		return err
	}

	workspaceInfo, err := api.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}

	if currentUser.ID != workspaceInfo.User.ID {
		return ErrInvalidUser
	}

	return nil
}
