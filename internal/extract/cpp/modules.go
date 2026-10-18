package cpp

import (
	"strconv"
	"strings"
)

type moduleDirective struct {
	Kind, Name string
	Start, End int
	Provider   bool
}

func parseModuleDirective(tokens []Token, start int) (moduleDirective, bool) {
	d := moduleDirective{Start: start}
	i := start
	exported := false
	if i < len(tokens) && tokens[i].Text == "export" {
		exported = true
		i++
	}
	if i >= len(tokens) || (tokens[i].Text != "module" && tokens[i].Text != "import") {
		return d, false
	}
	word := tokens[i].Text
	i++
	end := i
	for end < len(tokens) && tokens[end].Text != ";" {
		if tokens[end].Text == "{" || tokens[end].Text == "}" || tokens[end].Text == "=" {
			return d, false
		}
		end++
	}
	if end >= len(tokens) {
		return d, false
	}
	d.End = end
	if word == "module" && end == i && !exported {
		d.Kind = "global-fragment"
		return d, true
	}
	if word == "module" && end == i+2 && tokens[i].Text == ":" && tokens[i+1].Text == "private" && !exported {
		d.Kind = "private-fragment"
		return d, true
	}
	if word == "import" && end == i+1 && tokens[i].Kind == StringLiteral {
		v, e := strconv.Unquote(tokens[i].Text)
		if e != nil {
			return d, false
		}
		d.Name = v
		d.Kind = "header-quote"
		return d, true
	}
	if word == "import" && i < end && tokens[i].Text == "<" && tokens[end-1].Text == ">" {
		var p []string
		for _, t := range tokens[i+1 : end-1] {
			p = append(p, t.Text)
		}
		d.Name = strings.Join(p, "")
		d.Kind = "header-angle"
		return d, d.Name != ""
	}
	var name strings.Builder
	wantName := true
	partition := false
	for j := i; j < end; j++ {
		t := tokens[j]
		if wantName {
			if j == i && word == "import" && t.Text == ":" {
				name.WriteString(":")
				partition = true
				continue
			}
			if t.Kind != Identifier {
				return d, false
			}
			name.WriteString(t.Text)
			wantName = false
			continue
		}
		if t.Text == "." {
			name.WriteByte('.')
			wantName = true
			continue
		}
		if t.Text == ":" && !partition {
			name.WriteByte(':')
			partition = true
			wantName = true
			continue
		}
		return d, false
	}
	if wantName {
		return d, false
	}
	d.Name = name.String()
	if word == "module" {
		d.Kind = "declaration"
		d.Provider = exported || partition
	} else {
		d.Kind = "import"
	}
	return d, true
}
func (p *parser) parseModules() {
	ts := make([]Token, 0, len(p.sig))
	for _, raw := range p.sig {
		ts = append(ts, p.tokens[raw])
	}
	depth := 0
	current := ""
	for i := 0; i < len(ts); i++ {
		if ts[i].Text == "{" {
			depth++
			continue
		}
		if ts[i].Text == "}" {
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth != 0 {
			continue
		}
		if i > 0 && ts[i-1].Text != ";" && ts[i-1].Text != "}" {
			continue
		}
		d, ok := parseModuleDirective(ts, i)
		if !ok {
			continue
		}
		switch d.Kind {
		case "declaration":
			current = strings.SplitN(d.Name, ":", 2)[0]
			p.result.Modules = append(p.result.Modules, include{Target: d.Name, Source: "cpp:module-declaration"})
			if d.Provider {
				p.addDeclaration(&declaration{Kind: "module", Name: d.Name, Qualified: d.Name, Start: p.sig[i], HeaderEnd: ts[d.End].EndByte, End: ts[d.End].EndByte, BodyOpen: -1, BodyClose: -1})
			}
		case "import":
			if strings.HasPrefix(d.Name, ":") {
				if current == "" {
					i = d.End
					continue
				}
				d.Name = current + d.Name
			}
			p.result.Modules = append(p.result.Modules, include{Target: d.Name, Source: "cpp:module-import"})
		case "header-quote":
			p.result.Includes = append(p.result.Includes, include{Target: d.Name, Source: "cpp:include:quote"})
		case "header-angle":
			p.result.Includes = append(p.result.Includes, include{Target: d.Name, Source: "cpp:include:angle"})
		}
		i = d.End
	}
}
