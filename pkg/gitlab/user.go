package gitlab

import (
	"context"
)

type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
}

func (c *Client) GetUserInfo(ctx context.Context) (*User, error) {
	var query struct {
		CurrentUser *User `graphql:"currentUser"`
	}

	err := c.gqlClient.Query(ctx, &query, nil)
	if err != nil {
		return nil, err
	}

	return query.CurrentUser, nil
}
