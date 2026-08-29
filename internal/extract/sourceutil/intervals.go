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
