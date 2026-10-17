package evalcli

import (
	"bytes"
	"context"
	"testing"
)

func TestF14DiagnosticStreams(t *testing.T) {
	for _, tc := range []struct {
		arg  string
		code int
	}{{"--unknown", 1}, {"--help", 0}} {
		t.Run(tc.arg, func(t *testing.T) {
			var out, err bytes.Buffer
			got := Run(context.Background(), []string{tc.arg}, &out, &err)
			if got != tc.code {
				t.Errorf("code=%d want %d", got, tc.code)
			}
			if tc.code != 0 && out.Len() != 0 {
				t.Errorf("diagnostic stdout=%q", out.String())
			}
			if tc.code == 0 && (out.Len() == 0 || err.Len() != 0) {
				t.Errorf("help stdout=%q stderr=%q", out.String(), err.String())
			}
		})
	}
}

func TestF14JSONUnknownFlagKeepsStdoutEmpty(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"--json", "--unknown-option"}, &out, &errOut); code == 0 || out.Len() != 0 || errOut.Len() == 0 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
}
