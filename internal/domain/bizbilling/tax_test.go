package bizbilling

import (
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func TestBizExplicitZeroTaxPreservesLegacyDefault(t *testing.T) {
	items := []InvoiceLineItem{{Description: "service", Quantity: 1, UnitPrice: 10000}}
	legacy := ComputeInvoiceTotals(append([]InvoiceLineItem{}, items...), TaxConfig{}, "")
	require.Equal(t, int64(1800), legacy.TaxPaisa)
	zero := 0
	got, err := ComputeInvoiceTotalsChecked(items, TaxConfig{}, "", []LineTaxOverride{{Rate: &zero}})
	require.NoError(t, err)
	require.Zero(t, got.TaxPaisa)
	require.Equal(t, int64(10000), got.TotalPaisa)
	got, err = ComputeInvoiceTotalsChecked(items, TaxConfig{}, "", nil)
	require.NoError(t, err)
	require.Equal(t, int64(1800), got.TaxPaisa)
}
func TestBizTaxValidationNeverMutatesOnFailure(t *testing.T) {
	items := []InvoiceLineItem{{Quantity: 1, UnitPrice: 10000}, {Quantity: 2, UnitPrice: math.MaxInt64}}
	before := append([]InvoiceLineItem{}, items...)
	_, err := ComputeInvoiceTotalsChecked(items, TaxConfig{}, "", nil)
	require.Error(t, err)
	require.Equal(t, before, items)
	for _, item := range []InvoiceLineItem{{Quantity: -1, UnitPrice: 100}, {Quantity: 1, UnitPrice: -1}, {Quantity: 1, UnitPrice: 100, DiscountPaisa: 101}, {Quantity: 1, UnitPrice: 100, TaxRate: -1}} {
		_, err := ComputeInvoiceTotalsChecked([]InvoiceLineItem{item}, TaxConfig{}, "", nil)
		require.Error(t, err)
	}
}
func TestBizInclusiveTaxAndOddSplit(t *testing.T) {
	items := []InvoiceLineItem{{Quantity: 1, UnitPrice: 10000}}
	got, err := ComputeInvoiceTotalsChecked(items, TaxConfig{TaxInclusive: true, BusinessState: "KA"}, "ka", nil)
	require.NoError(t, err)
	require.Equal(t, int64(10000), got.TotalPaisa)
	require.Equal(t, got.TaxPaisa, got.Breakdown.CGSTPaisa+got.Breakdown.SGSTPaisa)
}
