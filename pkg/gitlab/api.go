package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hasura/go-graphql-client"
	"go.uber.org/zap"
)

type API interface {
	GetUserInfo(ctx context.Context) (*User, error)
	GetWorkspace(ctx context.Context, workspaceID string) (*Workspace, error)
}

type APIFactory func(accessToken string) API

type Client struct {
	accessToken string
	baseURL     string
	tokenType   TokenType
	gqlClient   *graphql.Client
}

type TokenType int

const (
	BearerTokenType TokenType = iota
	PrivateTokenType
)

var ErrWorkspaceNotFound = errors.New("workspace not found")

type tokenTransport struct {
	Transport http.RoundTripper
	Token     string
	tokenType TokenType
	logger    *zap.Logger
}

func (att tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Normal operation will use the Bearer token. Private token is enabled for the purposes
	// of running integration tests using the PAT
	if att.tokenType == BearerTokenType {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", att.Token))
	} else {
		req.Header.Add("PRIVATE-TOKEN", att.Token)
	}

	return att.Transport.RoundTrip(req)
}

func NewClient(logger *zap.Logger, accessToken, baseURL string, tokenType TokenType) *Client {
	client := &http.Client{
		Transport: http.DefaultTransport,
	}
	client.Transport = tokenTransport{
		logger:    logger,
		tokenType: tokenType,
		Transport: client.Transport,
		Token:     accessToken,
	}

	gqlClient := graphql.NewClient(fmt.Sprintf("%s/api/graphql", baseURL), client)

	return &Client{
		accessToken: accessToken,
		baseURL:     baseURL,
		tokenType:   tokenType,
		gqlClient:   gqlClient,
	}
}
