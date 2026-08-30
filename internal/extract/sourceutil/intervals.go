package sourceutil

import "sort"

type Interval struct {
	Start int
	End   int
}

func (i Interval) Valid() bool { return i.Start >= 0 && i.End >= i.Start }

func MergeIntervals(values []Interval) []Interval {
	ordered := make([]Interval, 0, len(values))
	for _, value := range values {
		if value.Valid() && value.End > value.Start {
			ordered = append(ordered, value)
		}
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Start != ordered[j].Start {
			return ordered[i].Start < ordered[j].Start
		}
		return ordered[i].End < ordered[j].End
	})
	merged := make([]Interval, 0, len(ordered))
	for _, value := range ordered {
		if len(merged) == 0 || value.Start > merged[len(merged)-1].End {
			merged = append(merged, value)
			continue
		}
		if value.End > merged[len(merged)-1].End {
			merged[len(merged)-1].End = value.End
		}
	}
	return merged
}

func SubtractIntervals(values, cuts []Interval) []Interval {
	remaining := MergeIntervals(values)
	for _, cut := range MergeIntervals(cuts) {
		next := make([]Interval, 0, len(remaining)+1)
		for _, value := range remaining {
			if cut.End <= value.Start || cut.Start >= value.End {
				next = append(next, value)
				continue
			}
			if value.Start < cut.Start {
				next = append(next, Interval{Start: value.Start, End: cut.Start})
			}
			if cut.End < value.End {
				next = append(next, Interval{Start: cut.End, End: value.End})
			}
		}
		remaining = next
	}
	return remaining
}

// Merge coalesces overlapping or adjacent half-open source spans. Invalid and
// empty spans are ignored, and the returned order is deterministic.
func Merge(values []Span) []Span {
	ordered := make([]Span, 0, len(values))
	for _, value := range values {
		if value.StartByte >= 0 && value.EndByte > value.StartByte {
			ordered = append(ordered, value)
		}
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].StartByte != ordered[j].StartByte {
			return ordered[i].StartByte < ordered[j].StartByte
		}
		if ordered[i].EndByte != ordered[j].EndByte {
			return ordered[i].EndByte < ordered[j].EndByte
		}
		return ordered[i].StartLine < ordered[j].StartLine
	})
	merged := make([]Span, 0, len(ordered))
	for _, value := range ordered {
		if len(merged) == 0 || value.StartByte > merged[len(merged)-1].EndByte {
			merged = append(merged, value)
			continue
		}
		last := &merged[len(merged)-1]
		if value.EndByte > last.EndByte {
			last.EndByte = value.EndByte
		}
		if value.StartLine > 0 && (last.StartLine == 0 || value.StartLine < last.StartLine) {
			last.StartLine = value.StartLine
		}
		if value.EndLine > last.EndLine {
			last.EndLine = value.EndLine
		}
	}
	return merged
}

// Subtract returns the byte fragments of whole not covered by covered. Cuts
// are clipped to whole and fragments retain whole's line envelope because a
// byte-only subtraction has no source map from which to derive new line ends.
func Subtract(whole Span, covered []Span) []Span {
	if whole.StartByte < 0 || whole.EndByte <= whole.StartByte {
		return nil
	}
	remaining := []Span{whole}
	for _, cut := range Merge(covered) {
		if cut.EndByte <= whole.StartByte || cut.StartByte >= whole.EndByte {
			continue
		}
		if cut.StartByte < whole.StartByte {
			cut.StartByte = whole.StartByte
		}
		if cut.EndByte > whole.EndByte {
			cut.EndByte = whole.EndByte
		}
		next := make([]Span, 0, len(remaining)+1)
		for _, value := range remaining {
			if cut.EndByte <= value.StartByte || cut.StartByte >= value.EndByte {
				next = append(next, value)
				continue
			}
			if value.StartByte < cut.StartByte {
				fragment := value
				fragment.EndByte = cut.StartByte
				next = append(next, fragment)
			}
			if cut.EndByte < value.EndByte {
				fragment := value
				fragment.StartByte = cut.EndByte
				next = append(next, fragment)
			}
		}
		remaining = next
	}
	return remaining
}
