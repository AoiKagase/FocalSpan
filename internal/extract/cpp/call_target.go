package cpp

import "strings"

type callTargetKind uint8

const (
	unqualifiedCall callTargetKind = iota
	namespaceCall
	memberCall
)

type callTarget struct {
	Kind            callTargetKind
	Name, Qualified string
	Absolute        bool
}

// parseCallTarget receives the significant token preceding the argument list.
func parseCallTarget(tokens []Token, beforeParen int) (callTarget, bool) {
	part := func(end int) (string, int, bool) {
		if end < 0 {
			return "", end, false
		}
		if tokens[end].Text == ">" || tokens[end].Text == ">>" {
			depth := 0
			for end >= 0 {
				v := tokens[end].Text
				if v == ">" {
					depth++
				}
				if v == ">>" {
					depth += 2
				}
				if v == "<" {
					depth--
				}
				if depth == 0 {
					end = previousSignificant(tokens, end)
					break
				}
				end = previousSignificant(tokens, end)
			}
		}
		if end < 0 || tokens[end].Kind != Identifier {
			return "", end, false
		}
		return tokens[end].Text, end, true
	}
	name, start, ok := part(beforeParen)
	if !ok {
		return callTarget{}, false
	}
	result := callTarget{Name: name}
	full := name
	for i := previousSignificant(tokens, start); i >= 0; {
		op := tokens[i].Text
		if op != "::" && op != "." && op != "->" {
			break
		}
		left := previousSignificant(tokens, i)
		v, j, found := part(left)
		if !found {
			if op == "::" {
				result.Absolute = true
				full = "::" + full
				result.Kind = namespaceCall
			}
			break
		}
		full = v + op + full
		if op == "." || op == "->" {
			result.Kind = memberCall
		} else if result.Kind != memberCall {
			result.Kind = namespaceCall
		}
		i = previousSignificant(tokens, j)
	}
	if result.Kind != unqualifiedCall {
		result.Qualified = full
	}
	result.Absolute = result.Absolute || strings.HasPrefix(full, "::")
	return result, true
}
