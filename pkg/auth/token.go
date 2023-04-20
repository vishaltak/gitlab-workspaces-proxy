package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

func getToken(ctx context.Context, config *Config, code string) (*token, error) {
	u := fmt.Sprintf("%s/oauth/token", config.Host)
	form := url.Values{
		"redirect_uri":  []string{config.RedirectURI},
		"grant_type":    []string{"authorization_code"},
		"code":          []string{code},
		"client_id":     []string{config.ClientID},
		"client_secret": []string{config.ClientSecret},
	}

	req, err := http.NewRequest(http.MethodPost, u, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var tkn token
	err = json.NewDecoder(res.Body).Decode(&tkn)
	if err != nil {
		return nil, err
	}

	return &tkn, nil
}

type token struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}
