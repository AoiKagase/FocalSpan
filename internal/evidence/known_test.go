package evidence

import (
	"strings"
	"testing"
)

func TestNormalizeKnownHandlesTrimsDeduplicatesAndPreservesOrder(t *testing.T) {
	got, err := NormalizeKnownHandles([]string{"  sym_a  ", "", "\t", "sym_b", "sym_a", " 日本語 ", "sym_c"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"sym_a", "sym_b", "日本語", "sym_c"}
	if len(got) != len(want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got=%v want=%v", got, want)
		}
	}
}

func TestNormalizeKnownHandlesBoundaries(t *testing.T) {
	valid256 := strings.Repeat("a", 256)
	values512 := make([]string, 512)
	for i := range values512 {
		values512[i] = "h" + strings.Repeat("x", i%20)
	}
	if _, err := NormalizeKnownHandles([]string{valid256}); err != nil {
		t.Fatalf("256-byte handle: %v", err)
	}
	if _, err := NormalizeKnownHandles(values512); err != nil {
		t.Fatalf("512 entries: %v", err)
	}

	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{name: "257 bytes", values: []string{strings.Repeat("a", 257)}, want: "256 bytes"},
		{name: "513 entries", values: make([]string, 513), want: "512"},
		{name: "nul", values: []string{"sym\x00bad"}, want: "control"},
		{name: "internal control", values: []string{"sym\nbad"}, want: "control"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NormalizeKnownHandles(tt.values)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v want %q", err, tt.want)
			}
		})
	}
}

func TestNormalizeKnownHandlesUsesUTF8ByteLimit(t *testing.T) {
	if _, err := NormalizeKnownHandles([]string{strings.Repeat("界", 85)}); err != nil {
		t.Fatalf("255 UTF-8 bytes: %v", err)
	}
	if _, err := NormalizeKnownHandles([]string{strings.Repeat("界", 86)}); err == nil {
		t.Fatal("258 UTF-8 bytes accepted")
	}
}
