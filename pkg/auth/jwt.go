package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func generateJwt(signingKey string, username string, expiresIn int) (string, error) {
	expirationTime := time.Now().Add(time.Duration(expiresIn) * time.Second)

	// Create the JWT claims, which includes the username and expiry time
	claims := &Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			// In JWT, the expiry time is expressed as unix milliseconds
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(signingKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func validateJwt(signingKey, token string) bool {
	var claims Claims
	tkn, err := jwt.ParseWithClaims(token, &claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(signingKey), nil
	})
	if err != nil {
		return false
	}

	if !tkn.Valid {
		return false
	}

	return err == nil
}
