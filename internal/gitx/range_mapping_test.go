package gitx

import (
	"reflect"
	"testing"
)

func TestMapLineRanges(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input []LineRange
		edits []lineEdit
		want  []LineRange
	}{{"insert above", []LineRange{{4, 4}}, []lineEdit{{0, 0, 1, 200}}, []LineRange{{204, 204}}}, {"insert inside", []LineRange{{2, 4}}, []lineEdit{{2, 0, 3, 2}}, []LineRange{{2, 6}}}, {"replace", []LineRange{{4, 4}}, []lineEdit{{3, 3, 3, 2}}, []LineRange{{3, 4}}}, {"delete inside", []LineRange{{3, 3}}, []lineEdit{{3, 2, 2, 0}}, []LineRange{{2, 2}}}, {"delete above", []LineRange{{4, 4}}, []lineEdit{{1, 2, 0, 0}}, []LineRange{{2, 2}}}, {"unchanged", []LineRange{{2, 3}}, nil, []LineRange{{2, 3}}}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := mapLineRanges(tc.input, tc.edits)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got=%+v want=%+v", got, tc.want)
			}
		})
	}
}
func TestMapLineRangesRejectsOverlappingEdits(t *testing.T) {
	if _, err := mapLineRanges([]LineRange{{1, 10}}, []lineEdit{{3, 3, 3, 1}, {4, 1, 4, 1}}); err == nil {
		t.Fatal("overlapping edits accepted")
	}
}
