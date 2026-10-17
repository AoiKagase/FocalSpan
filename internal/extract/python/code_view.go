package python

// Analysis uses non-code spans while preserving original byte/line coordinates.
func pythonCodeView(source []byte, tokens []Token) []byte {
	code := append([]byte(nil), source...)
	for _, token := range tokens {
		switch token.Kind {
		case String, TripleString, FString, Comment, TypeComment:
			for at := token.StartByte; at < token.EndByte && at < len(code); at++ {
				if code[at] != '\r' && code[at] != '\n' {
					code[at] = ' '
				}
			}
		}
	}
	return code
}
