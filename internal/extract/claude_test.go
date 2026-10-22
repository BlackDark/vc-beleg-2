package extract

import (
	"slices"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestConvert(t *testing.T) {
	ex := convert(answer{Date: ptr("2026-10-05"), Total: ptr(7.89), Merchant: ptr(" REWE "), Uncertain: []string{"merchant"}})
	if ex.Date != "2026-10-05" || ex.Total != 789 || ex.Merchant != "REWE" || !slices.Equal(ex.Uncertain, []string{"merchant"}) {
		t.Fatalf("convert = %+v", ex)
	}
	ex = convert(answer{Date: ptr("05.10.2026"), Total: ptr(-1.0)})
	if ex.Date != "" || ex.Total != 0 || !slices.Contains(ex.Uncertain, "date") || !slices.Contains(ex.Uncertain, "total") {
		t.Fatalf("implausible values kept: %+v", ex)
	}
	if ex := convert(answer{Total: ptr(0.1 + 0.2)}); ex.Total != 30 {
		t.Fatalf("rounding: %d", ex.Total)
	}
}
