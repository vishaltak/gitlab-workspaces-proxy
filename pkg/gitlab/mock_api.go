package gitlab

import (
	"context"
	"errors"
	"fmt"
)

type MockAPI struct {
	GetUserInfoUserID  int
	GetWorkspaceUserID int
	ValidToken         string
	AccessToken        string
}

func MockAPIFactory(accessToken string) API {
	return &MockAPI{
		GetUserInfoUserID:  1,
		GetWorkspaceUserID: 1,
		ValidToken:         accessToken,
		AccessToken:        accessToken,
	}
}

func (m *MockAPI) GetUserInfo(_ context.Context) (*User, error) {
	err := m.validateToken()
	if err != nil {
		return nil, err
	}

	return &User{
		ID:       fmt.Sprintf("gid://gitlab/User/%d", m.GetUserInfoUserID),
		Name:     "test",
		Username: "test",
	}, nil
}

func (m *MockAPI) GetWorkspace(_ context.Context, workspaceID string) (*Workspace, error) {
	err := m.validateToken()
	if err != nil {
		return nil, err
	}

	return &Workspace{
		ID:   workspaceID,
		Name: "test",
		User: User{
			ID:       fmt.Sprintf("gid://gitlab/User/%d", m.GetWorkspaceUserID),
			Username: "test",
		},
	}, nil
}

var ErrInvalidTokenError = errors.New("invalid token")

func (m *MockAPI) validateToken() error {
	if m.AccessToken != m.ValidToken {
		return ErrInvalidTokenError
	}
	return nil
}
