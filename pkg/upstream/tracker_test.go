package upstream

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestUpstreamTracker(t *testing.T) {
	tests := []struct {
		description       string
		upstreamToFind    string
		upstreamsToAdd    []HostMapping
		upstreamsToDelete []string
		expectedError     bool
		expectedHostName  string
	}{
		{
			description:       "When no upstreams are present returns error",
			upstreamToFind:    "test",
			upstreamsToAdd:    []HostMapping{},
			upstreamsToDelete: []string{},
			expectedError:     true,
			expectedHostName:  "",
		},
		{
			description:       "When upstreams is added, can return that upstream",
			upstreamToFind:    "test",
			upstreamsToAdd:    []HostMapping{{Hostname: "test"}},
			upstreamsToDelete: []string{},
			expectedError:     false,
			expectedHostName:  "test",
		},
		{
			description:       "When upstream is deleted, cannot find that upstream",
			upstreamToFind:    "test",
			upstreamsToAdd:    []HostMapping{{Hostname: "test"}},
			upstreamsToDelete: []string{"test"},
			expectedError:     true,
			expectedHostName:  "",
		},
		{
			description:       "When multiple upstreams are added and one is deleted, can find that upstream",
			upstreamToFind:    "test1",
			upstreamsToAdd:    []HostMapping{{Hostname: "test"}, {Hostname: "test1"}},
			upstreamsToDelete: []string{"test"},
			expectedError:     false,
			expectedHostName:  "test1",
		},
	}

	for _, tr := range tests {
		tracker := NewTracker(zaptest.NewLogger(t))
		t.Run(tr.description, func(t *testing.T) {
			for _, e := range tr.upstreamsToAdd {
				tracker.Add(e)
			}

			for _, e := range tr.upstreamsToDelete {
				tracker.DeleteByHostname(e)
			}

			result, err := tracker.GetByHostname(tr.upstreamToFind)
			if tr.expectedError {
				require.NotNil(t, err)
				return
			}

			require.Nil(t, err)
			require.Equal(t, tr.expectedHostName, result.Hostname)
		})
	}
}

func TestUpstreamTrackerGetByName(t *testing.T) {
	tests := []struct {
		description           string
		upstreamToFind        string
		upstreamsToAdd        []HostMapping
		upstreamsToDelete     []string
		expectedError         bool
		expectedWorkspaceName string
	}{
		{
			description:           "When no upstreams are present returns error",
			upstreamToFind:        "test",
			upstreamsToAdd:        []HostMapping{},
			upstreamsToDelete:     []string{},
			expectedError:         true,
			expectedWorkspaceName: "",
		},
		{
			description:           "When upstreams is added, can return that upstream",
			upstreamToFind:        "test_name",
			upstreamsToAdd:        []HostMapping{{Hostname: "test", WorkspaceName: "test_name"}},
			upstreamsToDelete:     []string{},
			expectedError:         false,
			expectedWorkspaceName: "test_name",
		},
		{
			description:           "When upstream is deleted, cannot find that upstream",
			upstreamToFind:        "test_name",
			upstreamsToAdd:        []HostMapping{{Hostname: "test", WorkspaceName: "test_name"}},
			upstreamsToDelete:     []string{"test"},
			expectedError:         true,
			expectedWorkspaceName: "",
		},
		{
			description:           "When multiple upstreams are added and one is deleted, can find that upstream",
			upstreamToFind:        "test1",
			upstreamsToAdd:        []HostMapping{{Hostname: "test_host", WorkspaceName: "test"}, {Hostname: "test1_host", WorkspaceName: "test1"}},
			upstreamsToDelete:     []string{"test_host"},
			expectedError:         false,
			expectedWorkspaceName: "test1",
		},
	}

	for _, tr := range tests {
		tracker := NewTracker(zaptest.NewLogger(t))
		t.Run(tr.description, func(t *testing.T) {
			for _, e := range tr.upstreamsToAdd {
				tracker.Add(e)
			}

			for _, e := range tr.upstreamsToDelete {
				tracker.DeleteByHostname(e)
			}

			result, err := tracker.GetByWorkspaceName(tr.upstreamToFind)
			if tr.expectedError {
				require.NotNil(t, err)
				return
			}

			require.Nil(t, err)
			require.Equal(t, tr.expectedWorkspaceName, result.WorkspaceName)
		})
	}
}
