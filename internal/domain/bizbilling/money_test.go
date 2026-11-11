package bizbilling

import (
	"github.com/stretchr/testify/require"
	"math"
	"math/big"
	"testing"
)

func TestBizMoneyOverflow(t *testing.T) {
	_, err := CheckedAdd(math.MaxInt64, 1)
	require.Error(t, err)
	_, err = CheckedAdd(math.MinInt64, -1)
	require.Error(t, err)
	_, err = CheckedMul(math.MinInt64, -1)
	require.Error(t, err)
	_, err = CheckedMul(math.MaxInt64, 2)
	require.Error(t, err)
	got, err := CheckedAdd(445, -10)
	require.NoError(t, err)
	require.Equal(t, int64(435), got)
	got, err = ProrateMinor(math.MaxInt64, 2, 2)
	require.NoError(t, err)
	require.Equal(t, int64(math.MaxInt64), got)
}
func FuzzBizCheckedArithmetic(f *testing.F) {
	for _, seed := range [][2]int64{{0, 0}, {math.MaxInt64, 1}, {math.MinInt64, -1}, {445, -10}, {1 << 32, 1 << 32}} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, a, b int64) {
		check := func(got int64, err error, want *big.Int) {
			t.Helper()
			if want.IsInt64() {
				require.NoError(t, err)
				require.Equal(t, want.Int64(), got)
			} else {
				require.Error(t, err)
			}
		}
		sum, err := CheckedAdd(a, b)
		check(sum, err, new(big.Int).Add(big.NewInt(a), big.NewInt(b)))
		product, err := CheckedMul(a, b)
		check(product, err, new(big.Int).Mul(big.NewInt(a), big.NewInt(b)))
	})
}
