package sourceutil

// BoundedLineWindows returns deterministic, one-based inclusive line windows.
// Windows overlap by at most overlap lines and never contain more than maxLines.
func BoundedLineWindows(lineCount, maxLines, overlap int) [][2]int {
	if lineCount <= 0 {
		return nil
	}
	if maxLines <= 0 {
		maxLines = 80
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= maxLines {
		overlap = maxLines - 1
	}
	step := maxLines - overlap
	result := make([][2]int, 0, (lineCount+step-1)/step)
	for start := 1; start <= lineCount; start += step {
		end := start + maxLines - 1
		if end > lineCount {
			end = lineCount
		}
		result = append(result, [2]int{start, end})
		if end == lineCount {
			break
		}
	}
	return result
}
