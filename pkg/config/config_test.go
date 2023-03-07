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
		expectedAuthClientId string
	}{
		{
			description:          "When invalid filename is passed throws error",
			filename:             "./fixtures/error.yaml",
			expectedError:        true,
			expectedAuthClientId: "",
		},
		{
			description:          "When a valid filename is passed loads config",
			filename:             "./fixtures/sample.yaml",
			expectedError:        false,
			expectedAuthClientId: "CLIENT_ID",
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
			require.Equal(t, tr.expectedAuthClientId, config.Auth.ClientID)
		})
	}

}