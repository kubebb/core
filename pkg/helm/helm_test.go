package helm

import (
	"fmt"
	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/chart"
	"testing"
)

// TestPull for HelmWrapper.Pull
func TestPull(t *testing.T) {
	testCases := []struct {
		description string
		url         string

		expected *chart.Chart
	}{}
	logger := logr.Logger{}
	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("test: %s", testCase.description), func(t *testing.T) {
			logger.Info(fmt.Sprint(testCase.expected))
		})
	}
}
