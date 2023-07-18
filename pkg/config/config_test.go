package config

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLoadConfig(t *testing.T) {
	tt := []struct {
		description          string
		filename             string
		expectedError        bool
		expectedAuthClientID string
		expectedMetricsPath  string
		expectedHTTPPort     int
		expectedLogLevel     string
		expectedSSHPort      int
		expectedSSHEnabled   bool
	}{
		{
			description:          "When invalid filename is passed throws error",
			filename:             "./fixtures/error.yaml",
			expectedError:        true,
			expectedAuthClientID: "",
			expectedLogLevel:     "info",
		},
		{
			description:          "When a valid filename is passed loads config",
			filename:             "./fixtures/sample.yaml",
			expectedError:        false,
			expectedAuthClientID: "CLIENT_ID",
			expectedMetricsPath:  "/v1/metrics",
			expectedHTTPPort:     1234,
			expectedLogLevel:     "info",
		},
		{
			description:          "When metrics path is not present in config, defaults metrics path",
			filename:             "./fixtures/sample_without_metrics.yaml",
			expectedError:        false,
			expectedAuthClientID: "CLIENT_ID",
			expectedMetricsPath:  "/metrics",
			expectedHTTPPort:     1234,
			expectedLogLevel:     "info",
		},
		{
			description:          "When port is not present in config defaults port",
			filename:             "./fixtures/sample_without_port.yaml",
			expectedError:        false,
			expectedAuthClientID: "CLIENT_ID",
			expectedMetricsPath:  "/metrics",
			expectedHTTPPort:     9876,
			expectedLogLevel:     "info",
		},
		{
			description:   "When client id is missing from config throws error",
			filename:      "./fixtures/sample_missing_client_id.yaml",
			expectedError: true,
		},
		{
			description:          "When log level is present in config loads level",
			filename:             "./fixtures/sample_with_log_level.yaml",
			expectedError:        false,
			expectedAuthClientID: "CLIENT_ID",
			expectedMetricsPath:  "/metrics",
			expectedHTTPPort:     9876,
			expectedLogLevel:     "debug",
		},
		{
			description:          "When the SSH section is present, sets values",
			filename:             "./fixtures/sample_with_ssh_details.yaml",
			expectedError:        false,
			expectedAuthClientID: "CLIENT_ID",
			expectedMetricsPath:  "/v1/metrics",
			expectedHTTPPort:     1234,
			expectedLogLevel:     "info",
			expectedSSHPort:      2222,
			expectedSSHEnabled:   true,
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
			require.Equal(t, tr.expectedHTTPPort, config.HTTP.Port)
			require.Equal(t, tr.expectedLogLevel, config.LogLevel)
			require.Equal(t, tr.expectedSSHEnabled, config.SSH.Enabled)
			require.Equal(t, tr.expectedSSHPort, config.SSH.Port)
		})
	}
}

func TestGetZapLevel(t *testing.T) {
	tt := []struct {
		description    string
		logLevel       string
		expectedError  bool
		expectedResult zap.AtomicLevel
	}{
		{
			description:    "When valid level is passed returns zap level",
			logLevel:       "debug",
			expectedError:  false,
			expectedResult: zap.NewAtomicLevelAt(zap.DebugLevel),
		},
		{
			description:   "When invalid level is passed returns error",
			logLevel:      "blah",
			expectedError: true,
		},
	}

	for _, tr := range tt {
		t.Run(tr.description, func(t *testing.T) {
			config := &Config{
				LogLevel: tr.logLevel,
			}

			result, err := config.GetZapLevel()
			if tr.expectedError {
				require.Error(t, err)
				return
			}

			require.Equal(t, tr.expectedResult, result)
		})
	}
}
