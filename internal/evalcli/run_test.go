package evalcli

import "testing"

func TestParseAblation(t *testing.T) {
	modes, err := parseAblation("all")
	if err != nil || len(modes) != 3 {
		t.Fatalf("modes=%v err=%v", modes, err)
	}
	if _, err := parseAblation("unknown"); err == nil {
		t.Fatal("unknown ablation accepted")
	}
}
