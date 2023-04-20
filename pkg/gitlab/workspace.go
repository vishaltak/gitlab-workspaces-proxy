package gitlab

import (
	"context"
	"fmt"
)

type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	User User   `json:"user"`
}

type RemoteDevelopmentWorkspaceID string

func (c *Client) GetWorkspace(ctx context.Context, workspaceID string) (*Workspace, error) {
	gid := fmt.Sprintf("gid://gitlab/RemoteDevelopment::Workspace/%s", workspaceID)
	var query struct {
		Workspace *Workspace `graphql:"workspace(id: $workspaceID)"`
	}

	err := c.gqlClient.Query(ctx, &query, map[string]interface{}{
		"workspaceID": RemoteDevelopmentWorkspaceID(gid),
	})
	if err != nil {
		return nil, err
	}

	if query.Workspace == nil {
		return nil, ErrWorkspaceNotFound
	}

	return query.Workspace, nil
}
