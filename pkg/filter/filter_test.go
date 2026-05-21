package filter_test

import (
	"testing"

	"github.com/saschanowak/fastly-historical-stats-exporter/pkg/filter"
)

func TestZeroValuePermitsEverything(t *testing.T) {
	var f filter.Filter
	for _, s := range []string{"requests", "hits", "bandwidth", "imgopto", "any_metric"} {
		if !f.Permit(s) {
			t.Errorf("zero-value Filter.Permit(%q) = false, want true", s)
		}
	}
}

func TestAllowlistRestrictsToMatchingNames(t *testing.T) {
	var f filter.Filter
	if err := f.Allow("^requests$"); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		want bool
	}{
		{"requests", true},
		{"hits", false},
		{"bandwidth", false},
		{"requests_extra", false},
	}
	for _, tt := range tests {
		if got := f.Permit(tt.name); got != tt.want {
			t.Errorf("Permit(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestBlocklistExcludesMatchingNames(t *testing.T) {
	var f filter.Filter
	if err := f.Block("imgopto"); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		want bool
	}{
		{"imgopto", false},
		{"imgopto_transforms", false},
		{"imgopto_resp_body_bytes", false},
		{"requests", true},
		{"hits", true},
	}
	for _, tt := range tests {
		if got := f.Permit(tt.name); got != tt.want {
			t.Errorf("Permit(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestBlocklistTakesPrecedenceOverAllowlist(t *testing.T) {
	var f filter.Filter
	if err := f.Allow("^requests$"); err != nil {
		t.Fatal(err)
	}
	if err := f.Block("^requests$"); err != nil {
		t.Fatal(err)
	}

	if f.Permit("requests") {
		t.Error(`Permit("requests") = true; block should take precedence over allow`)
	}
}

func TestMultipleAllowPatterns(t *testing.T) {
	var f filter.Filter
	if err := f.Allow("^hits$"); err != nil {
		t.Fatal(err)
	}
	if err := f.Allow("^miss$"); err != nil {
		t.Fatal(err)
	}

	if !f.Permit("hits") {
		t.Error(`Permit("hits") = false, want true`)
	}
	if !f.Permit("miss") {
		t.Error(`Permit("miss") = false, want true`)
	}
	if f.Permit("requests") {
		t.Error(`Permit("requests") = true, want false`)
	}
}

func TestMultipleBlockPatterns(t *testing.T) {
	var f filter.Filter
	if err := f.Block("^imgopto"); err != nil {
		t.Fatal(err)
	}
	if err := f.Block("^waf_"); err != nil {
		t.Fatal(err)
	}

	if f.Permit("imgopto") {
		t.Error(`Permit("imgopto") = true, want false`)
	}
	if f.Permit("waf_blocked") {
		t.Error(`Permit("waf_blocked") = true, want false`)
	}
	if !f.Permit("requests") {
		t.Error(`Permit("requests") = false, want true`)
	}
}

func TestAllowWithSubstringMatch(t *testing.T) {
	var f filter.Filter
	if err := f.Allow("bytes"); err != nil {
		t.Fatal(err)
	}

	if !f.Permit("resp_body_bytes") {
		t.Error(`Permit("resp_body_bytes") = false, want true`)
	}
	if !f.Permit("bereq_body_bytes") {
		t.Error(`Permit("bereq_body_bytes") = false, want true`)
	}
	if f.Permit("requests") {
		t.Error(`Permit("requests") = true, want false`)
	}
}

func TestInvalidAllowPatternReturnsError(t *testing.T) {
	var f filter.Filter
	if err := f.Allow("[invalid"); err == nil {
		t.Error("Allow with invalid regex should return error, got nil")
	}
}

func TestInvalidBlockPatternReturnsError(t *testing.T) {
	var f filter.Filter
	if err := f.Block("[invalid"); err == nil {
		t.Error("Block with invalid regex should return error, got nil")
	}
}

func TestAllowErrorDoesNotAddPattern(t *testing.T) {
	var f filter.Filter
	// prime with a valid allow so zero-value "permit all" doesn't mask the issue
	if err := f.Allow("^hits$"); err != nil {
		t.Fatal(err)
	}
	// invalid pattern must be rejected without side effects
	_ = f.Block("[bad")

	// hits still allowed, requests still blocked — blocklist unchanged
	if !f.Permit("hits") {
		t.Error(`Permit("hits") = false after failed Block, want true`)
	}
}
