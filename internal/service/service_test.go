package service

import (
	"testing"
	"time"

	"github.com/Samuel-DA/open-banking-financial-insights/internal/domain"
)

func TestCategorise(t *testing.T) {
	cases := map[string]string{"TESCO STORES 123": "groceries", "TfL Travel Charge": "transport", "ACME PAYROLL": "income", "Mystery Shop": "other"}
	for in, want := range cases {
		if got := Categorise(in, -100); got != want {
			t.Errorf("Categorise(%q)=%q want %q", in, got, want)
		}
	}
}
func TestSummarise(t *testing.T) {
	m := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	got := Summarise("a", m, []domain.Transaction{{Amount: 300000, Description: "salary"}, {Amount: -5500, Description: "tesco"}, {Amount: -2500, Description: "tfl"}})
	if got.TotalIncome != 300000 || got.TotalSpending != 8000 || got.NetCashflow != 292000 || got.CategorySpend["groceries"] != 5500 {
		t.Fatalf("unexpected summary: %+v", got)
	}
}
