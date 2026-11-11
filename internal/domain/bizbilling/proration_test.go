package bizbilling

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBizProrationTruncatesMinorUnits(t *testing.T) {
	got, err := ProrateMinor(100, 1, 3)
	require.NoError(t, err)
	require.Equal(t, int64(33), got)
	_, err = ProrateMinor(100, 1, 0)
	require.Error(t, err)
	_, err = ProrateMinor(-1, 1, 3)
	require.Error(t, err)
}
