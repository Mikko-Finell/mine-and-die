package effects

import "testing"

func TestNewBloodSplatterParamsReturnsCopy(t *testing.T) {
	params := NewBloodSplatterParams()
	if len(params) == 0 {
		t.Fatalf("expected params to contain defaults")
	}

	params["drag"] = 99
	params["new"] = 1

	fresh := NewBloodSplatterParams()
	if fresh["drag"] == 99 {
		t.Fatalf("expected drag default to remain unchanged, got %v", fresh["drag"])
	}
	if _, exists := fresh["new"]; exists {
		t.Fatalf("expected defaults to omit new keys, got %v", fresh)
	}
}

func TestBloodSplatterColorsReturnsCopy(t *testing.T) {
	colors := BloodSplatterColors()
	if len(colors) == 0 {
		t.Fatalf("expected default blood colors")
	}

	original := colors[0]
	colors[0] = "#ffffff"

	fresh := BloodSplatterColors()
	if fresh[0] != original {
		t.Fatalf("expected palette copy to remain unchanged, got %q", fresh[0])
	}
}
