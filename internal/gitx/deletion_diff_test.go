package gitx

import "testing"

func TestParseUnifiedZeroDiffDeletionBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, metadata, hunk string
		want                 []LineRange
	}{
		{"middle", "", "@@ -7,2 +6,0 @@", []LineRange{{Start: 6, End: 6}}},
		{"start", "", "@@ -1,2 +0,0 @@", []LineRange{{Start: 1, End: 1}}},
		{"eof_implicit_count", "", "@@ -10 +9,0 @@", []LineRange{{Start: 9, End: 9}}},
		{"empty", "", "@@ -0,0 +0,0 @@", nil},
		{"deleted_file", "deleted file mode 100644\n--- a/auth.go\n+++ /dev/null\n", "@@ -1,3 +0,0 @@", nil},
		{"insertion", "", "@@ -2,0 +3,2 @@", []LineRange{{Start: 3, End: 4}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseUnifiedZeroDiff([]byte("diff --git a/auth.go b/auth.go\n" + tc.metadata + tc.hunk + "\n"))
			if err != nil || len(got) != 1 {
				t.Fatalf("parse=%+v err=%v", got, err)
			}
			if len(got[0].Ranges) != len(tc.want) {
				t.Fatalf("ranges=%+v, want %+v", got[0].Ranges, tc.want)
			}
			for i, want := range tc.want {
				if got[0].Ranges[i] != want {
					t.Errorf("range=%+v, want %+v", got[0].Ranges[i], want)
				}
			}
		})
	}
}
