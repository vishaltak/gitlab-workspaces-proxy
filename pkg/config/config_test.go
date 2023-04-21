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
		expectedMetricsPath  string
		expectedPort         int
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
			expectedMetricsPath:  "/v1/metrics",
			expectedPort:         1234,
		},
		{
			description:          "When metrics path is not present in config",
			filename:             "./fixtures/sample_without_metrics.yaml",
			expectedError:        false,
			expectedAuthClientID: "CLIENT_ID",
			expectedMetricsPath:  "/metrics",
			expectedPort:         1234,
		},
		{
			description:          "When port is not present in config",
			filename:             "./fixtures/sample_without_port.yaml",
			expectedError:        false,
			expectedAuthClientID: "CLIENT_ID",
			expectedMetricsPath:  "/metrics",
			expectedPort:         9876,
		},
		{
			description:   "When port is not present in config",
			filename:      "./fixtures/sample_mssing_client_id.yaml",
			expectedError: true,
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
			require.Equal(t, tr.expectedMetricsPath, config.MetricsPath)
			require.Equal(t, tr.expectedPort, config.Port)
		})
	}
}
