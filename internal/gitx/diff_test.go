package gitx

import "testing"

func TestParseUnifiedZeroDiffKindsAndRanges(t *testing.T) {
	data := []byte("diff --git a/auth.go b/auth.go\nindex 1..2 100644\n--- a/auth.go\n+++ b/auth.go\n@@ -4,0 +5,3 @@\n+one\n+two\n+three\n\n" +
		"diff --git a/old.go b/new.go\nsimilarity index 90%\nrename from old.go\nrename to new.go\n" +
		"diff --git a/delete.go b/delete.go\ndeleted file mode 100644\n--- a/delete.go\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-old\n-file\n" +
		"diff --git a/image.bin b/image.bin\nnew file mode 100644\nindex 0..1\nBinary files /dev/null and b/image.bin differ\n")
	files, err := ParseUnifiedZeroDiff(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 4 {
		t.Fatalf("files=%+v", files)
	}
	if files[0].Path != "auth.go" || len(files[0].Ranges) != 1 || files[0].Ranges[0].Start != 5 || files[0].Ranges[0].End != 7 {
		t.Fatalf("first=%+v", files[0])
	}
	if files[1].Kind != ChangeRename || files[1].OldPath != "old.go" || files[1].Path != "new.go" {
		t.Fatalf("rename=%+v", files[1])
	}
	if files[2].Kind != ChangeDelete || !files[3].Binary {
		t.Fatalf("delete/binary=%+v", files)
	}
}

func TestParseUnifiedZeroDiffHandlesEmptyAndCRLF(t *testing.T) {
	data := []byte("diff --git a/empty.txt b/empty.txt\r\n--- a/empty.txt\r\n+++ b/empty.txt\r\n@@ -0,0 +0,0 @@\r\n")
	files, err := ParseUnifiedZeroDiff(data)
	if err != nil || len(files) != 1 {
		t.Fatalf("files=%+v err=%v", files, err)
	}
	if files[0].Kind != ChangeModify || len(files[0].Ranges) != 0 {
		t.Fatalf("empty=%+v", files[0])
	}
}
