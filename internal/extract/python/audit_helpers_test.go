package python

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func auditPython(t *testing.T, source string) model.Extraction {
	t.Helper()
	e, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "sample.py", Language: "python", Content: []byte(source)})
	if err != nil {
		t.Fatal(err)
	}
	return e
}
