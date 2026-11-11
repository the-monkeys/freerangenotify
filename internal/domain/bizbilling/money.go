package bizbilling

import (
	"errors"
	"math"
	"math/big"
)

var ErrMoneyOverflow = errors.New("billing amount exceeds supported minor-unit range")
var ErrInvalidMoney = errors.New("invalid billing amount or rate")

func CheckedAdd(a, b int64) (int64, error) {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		return 0, ErrMoneyOverflow
	}
	return a + b, nil
}
func CheckedMul(a, b int64) (int64, error) {
	if a == 0 || b == 0 {
		return 0, nil
	}
	if (a == math.MinInt64 && b == -1) || (b == math.MinInt64 && a == -1) {
		return 0, ErrMoneyOverflow
	}
	result := a * b
	if result/b != a {
		return 0, ErrMoneyOverflow
	}
	return result, nil
}

// ProrateMinor truncates toward a whole minor unit without intermediate overflow.
func ProrateMinor(amount, numerator, denominator int64) (int64, error) {
	if amount < 0 || numerator < 0 || denominator <= 0 {
		return 0, ErrInvalidMoney
	}
	result := new(big.Int).Mul(big.NewInt(amount), big.NewInt(numerator))
	result.Quo(result, big.NewInt(denominator))
	if !result.IsInt64() {
		return 0, ErrMoneyOverflow
	}
	return result.Int64(), nil
}

// A non-nil Rate explicitly overrides legacy zero-means-default behavior.
type LineTaxOverride struct {
	Rate   *int `json:"rate,omitempty"`
	Exempt bool `json:"exempt,omitempty"`
}
