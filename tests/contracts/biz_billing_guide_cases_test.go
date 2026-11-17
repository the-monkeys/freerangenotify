package contracts

import (
	"github.com/stretchr/testify/require"
	"testing"
)

// Executes the published requests through the actual authentication, handler
// and configuration service rather than asserting prose or copied DTO shapes.
func TestBizBillingGuideContracts(t *testing.T) {
	var cases []APICase
	for _, c := range loadCases(t) {
		if c.ID == "create-app" || c.Guide == "business-billing" {
			cases = append(cases, c)
		}
	}
	require.Len(t, cases, 5, "required billing examples must not silently disappear")
	runExamples(t, cases)
}
