package gitx

import "testing"

func TestParseUnifiedZeroDiffQuotedPaths(t *testing.T) {
	for _, tc := range []struct {
		name, data, oldPath, path string
	}{
		{"unicode", "diff --git \"a/\\350\\252\\215\\350\\250\\274.go\" \"b/\\350\\252\\215\\350\\250\\274.go\"\n--- \"a/\\350\\252\\215\\350\\250\\274.go\"\n+++ \"b/\\350\\252\\215\\350\\250\\274.go\"\n@@ -3 +3 @@\n", "認証.go", "認証.go"},
		{"escaped", "diff --git \"a/a\\t\\\"b.go\" \"b/a\\t\\\"b.go\"\n--- \"a/a\\t\\\"b.go\"\n+++ \"b/a\\t\\\"b.go\"\n@@ -3 +3 @@\n", "a\t\"b.go", "a\t\"b.go"},
		{"spaces", "diff --git a/auth config.go b/auth config.go\nold mode 100644\nnew mode 100755\n", "auth config.go", "auth config.go"},
		{"embedded_separator", "diff --git a/folder b/name.go b/folder b/name.go\nold mode 100644\nnew mode 100755\n", "folder b/name.go", "folder b/name.go"},
		{"rename", "diff --git \"a/old\\t.go\" \"b/new\\t.go\"\nsimilarity index 100%\nrename from \"old\\t.go\"\nrename to \"new\\t.go\"\n", "old\t.go", "new\t.go"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseUnifiedZeroDiff([]byte(tc.data))
			if err != nil || len(got) != 1 || got[0].Path != tc.path || got[0].OldPath != tc.oldPath {
				t.Fatalf("paths=%+v err=%v; want old=%q new=%q", got, err, tc.oldPath, tc.path)
			}
		})
	}
}

func TestParseUnifiedZeroDiffRejectsMalformedQuotedPath(t *testing.T) {
	if _, err := ParseUnifiedZeroDiff([]byte("diff --git a/auth.go b/auth.go\n+++ \"b/broken\\q.go\"\n")); err == nil {
		t.Fatal("malformed quoted path accepted")
	}
}

func TestParseUnifiedZeroDiffIgnoresHeaderLikeSourceLines(t *testing.T) {
	data := "diff --git a/auth.go b/auth.go\n--- a/auth.go\n+++ b/auth.go\n@@ -1 +1 @@\n--- \"not a valid path\\q\"\n+++ source text\n"
	got, err := ParseUnifiedZeroDiff([]byte(data))
	if err != nil || len(got) != 1 || got[0].Path != "auth.go" || got[0].OldPath != "auth.go" {
		t.Fatalf("source lines treated as Git metadata: %+v err=%v", got, err)
	}
}
