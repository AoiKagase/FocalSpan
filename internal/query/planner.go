package query

import "strings"

const maxAnchors = 8

type intentLexicon struct {
	intent   Intent
	words    []string
	phrases  []string
	japanese []string
}

var plannerIntents = []intentLexicon{
	{intent: IntentImpact, words: []string{"impact"}, phrases: []string{"affected", "blast radius", "what breaks"}, japanese: []string{"影響", "影響範囲", "波及", "壊れる箇所", "変更範囲", "何が壊れるか"}},
	{intent: IntentTests, words: []string{"test", "tests", "testing", "coverage", "spec"}, japanese: []string{"テスト", "検証コード", "カバレッジ", "試験"}},
	{intent: IntentCallers, words: []string{"caller", "callers", "usages", "used by"}, phrases: []string{"called by"}, japanese: []string{"呼び出し元", "使用箇所", "利用箇所", "どこから呼ばれる", "参照元"}},
	{intent: IntentCallees, words: []string{"callee", "callees"}, phrases: []string{"calls from", "what does", "dependencies called"}, japanese: []string{"呼び出し先", "何を呼ぶ", "内部で呼ぶ", "依存先"}},
	{intent: IntentImports, words: []string{"import", "imports", "include", "includes", "require", "extends", "layout", "partial", "template"}, japanese: []string{"読み込み", "読み込んでいる", "読み込む", "インポート", "インクルード", "継承元テンプレート", "部品テンプレート"}},
	{intent: IntentExports, words: []string{"export", "exports", "re-export"}, japanese: []string{"エクスポート", "再エクスポート", "公開元"}},
	{intent: IntentReferences, words: []string{"reference", "references", "implement", "implements", "interface", "inherits", "type usage"}, japanese: []string{"参照", "実装している", "継承", "型の使用箇所"}},
	{intent: IntentDefinition, words: []string{"define", "defined", "definition", "implementation", "declaration"}, phrases: []string{"where is"}, japanese: []string{"定義", "実装", "宣言", "どこにある", "場所"}},
}

var relationForIntent = map[Intent][]string{
	IntentCallers:    {"callers"},
	IntentCallees:    {"callees"},
	IntentTests:      {"tests"},
	IntentImports:    {"imports"},
	IntentExports:    {"exports"},
	IntentReferences: {"references"},
	IntentImpact:     {"callers", "tests", "references"},
}

func PlanQuery(raw string) Plan {
	terms := Normalize(raw)
	lower := strings.ToLower(raw)
	intents := make([]Intent, 0, len(plannerIntents))
	for _, entry := range plannerIntents {
		if intentMatches(lower, entry) {
			intents = append(intents, entry.intent)
		}
	}
	if len(intents) == 0 {
		intents = append(intents, IntentDefinition)
	}
	primary := intents[0]
	plan := Plan{
		RawQuery:      raw,
		Terms:         terms,
		Intents:       intents,
		PrimaryIntent: primary,
		Profile:       string(primary),
	}
	plan.Anchors = planAnchors(raw, terms)
	plan.Relations = planRelations(intents)
	return plan
}

func intentMatches(raw string, entry intentLexicon) bool {
	if explicitIntentMatches(raw, entry) {
		return true
	}
	if entry.intent == IntentCallers && containsEnglishTerm(raw, "call") || entry.intent == IntentCallers && containsEnglishTerm(raw, "calls") {
		for _, candidate := range plannerIntents {
			if candidate.intent == IntentCallees && explicitIntentMatches(raw, candidate) {
				return false
			}
		}
		return true
	}
	return false
}

func explicitIntentMatches(raw string, entry intentLexicon) bool {
	for _, word := range entry.words {
		if containsEnglishTerm(raw, word) {
			return true
		}
	}
	for _, phrase := range entry.phrases {
		if containsEnglishTerm(raw, phrase) {
			return true
		}
	}
	for _, phrase := range entry.japanese {
		if strings.Contains(raw, strings.ToLower(phrase)) {
			return true
		}
	}
	return false
}

func containsEnglishTerm(raw, term string) bool {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return false
	}
	start := 0
	for {
		index := strings.Index(raw[start:], term)
		if index < 0 {
			return false
		}
		index += start
		beforeOK := index == 0 || !isASCIITermRune(raw[index-1])
		end := index + len(term)
		afterOK := end == len(raw) || !isASCIITermRune(raw[end])
		if beforeOK && afterOK {
			return true
		}
		start = index + len(term)
		if start >= len(raw) {
			return false
		}
	}
}

func isASCIITermRune(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || value == '_'
}

func planAnchors(raw string, terms Terms) []string {
	anchors := make([]string, 0, maxAnchors)
	seen := make(map[string]bool, maxAnchors)
	add := func(value string) bool {
		value = strings.TrimSpace(value)
		if value == "" {
			return true
		}
		key := strings.ToLower(value)
		if seen[key] {
			return true
		}
		if len(anchors) >= maxAnchors {
			return false
		}
		seen[key] = true
		anchors = append(anchors, value)
		return true
	}
	for _, value := range terms.Identifiers {
		if isQualifiedIdentifier(value) {
			if !add(value) {
				return anchors
			}
		}
	}
	for _, value := range terms.Symbols {
		if !add(value) {
			return anchors
		}
	}
	for _, value := range terms.Identifiers {
		if !add(value) {
			return anchors
		}
	}
	for _, value := range terms.Paths {
		if !add(value) {
			return anchors
		}
	}
	for _, value := range terms.Phrases {
		if phraseLooksLikeCode(value) {
			if !add(value) {
				return anchors
			}
		}
	}
	for _, value := range terms.Words {
		if isIntentLexeme(value) || isJapaneseParticle(value) || !containsCodeLikeRune(value) {
			continue
		}
		if !add(value) {
			return anchors
		}
	}
	_ = raw
	return anchors
}

func planRelations(intents []Intent) []string {
	result := make([]string, 0)
	seen := make(map[string]bool)
	for _, intent := range intents {
		for _, relation := range relationForIntent[intent] {
			if !seen[relation] {
				seen[relation] = true
				result = append(result, relation)
			}
		}
	}
	return result
}

func isQualifiedIdentifier(value string) bool {
	return !looksLikePath(value) && strings.ContainsAny(value, ":.\\")
}

func phraseLooksLikeCode(value string) bool {
	return containsCodeLikeRune(value) || strings.ContainsAny(value, "/\\.-\"")
}

func containsCodeLikeRune(value string) bool {
	for _, r := range value {
		if r >= 'A' && r <= 'Z' || r == '_' || r == ':' || r == '.' || r == '\\' {
			return true
		}
	}
	return false
}

func isIntentLexeme(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, entry := range plannerIntents {
		for _, candidate := range append(append(append([]string{}, entry.words...), entry.phrases...), entry.japanese...) {
			if value == strings.ToLower(candidate) {
				return true
			}
		}
	}
	return false
}

func isJapaneseParticle(value string) bool {
	return value == "の" || value == "を" || value == "は" || value == "が" || value == "に" || value == "へ" || value == "と" || value == "で" || value == "や" || value == "も"
}
