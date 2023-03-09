package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	tt := []struct {
		description          string
		filename             string
		expectedError        bool
		expectedAuthClientID string
	}{
		{
			description:          "When invalid filename is passed throws error",
			filename:             "./fixtures/error.yaml",
			expectedError:        true,
			expectedAuthClientID: "",
		},
		{
			description:          "When a valid filename is passed loads config",
			filename:             "./fixtures/sample.yaml",
			expectedError:        false,
			expectedAuthClientID: "CLIENT_ID",
		},
	}

	for _, tr := range tt {
		t.Run(tr.description, func(t *testing.T) {
			config, err := LoadConfig(tr.filename)
			if tr.expectedError {
				require.NotNil(t, err)
				return
			}

			require.Nil(t, err)
			require.Equal(t, tr.expectedAuthClientID, config.Auth.ClientID)
		})
	}
}
