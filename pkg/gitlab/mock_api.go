package gitlab

import "context"

type MockAPI struct{}

func MockAPIFactory(accessToken string) API {
	return &MockAPI{}
}

func (m *MockAPI) GetUserInfo(_ context.Context) (*User, error) {
	return &User{
		ID:       "gid://gitlab/User/1",
		Name:     "test",
		Username: "test",
	}, nil
}

func (m *MockAPI) GetWorkspace(ctx context.Context, workspaceID string) (*Workspace, error) {
	return &Workspace{
		ID:   workspaceID,
		Name: "test",
		User: User{
			ID:       "gid://gitlab/User/1",
			Username: "test",
		},
	}, nil
}
