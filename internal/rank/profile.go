package rank

import "github.com/focalspan/focalspan/internal/query"

const fusionScale = 1000.0

// Profile contains the deterministic weights used after retrieval fusion.
// RelationWeights is copied for every caller so ranking cannot mutate shared
// profile state.
type Profile struct {
	Name                 string
	QualifiedExact       float64
	SymbolExact          float64
	Prefix               float64
	NaturalPrefix        float64
	LexicalMax           float64
	PathMax              float64
	ChangedFile          float64
	TestMatch            float64
	NonTestPenalty       float64
	DocumentationPenalty float64
	LargeSpanPenalty     float64
	RelationWeights      map[string]float64
}

var relationWeightsByIntent = map[query.Intent]map[string]float64{
	query.IntentDefinition: {"callers": 30, "callees": 30, "tests": 40, "imports": 40, "exports": 40, "references": 40},
	query.IntentCallers:    {"callers": 500, "tests": 180, "callees": 60, "imports": 40, "exports": 40, "references": 60},
	query.IntentCallees:    {"callees": 500, "callers": 60, "tests": 60, "imports": 40, "exports": 40, "references": 60},
	query.IntentTests:      {"tests": 500, "callers": 160, "callees": 60, "imports": 40, "exports": 40, "references": 60},
	query.IntentImports:    {"imports": 500, "exports": 420, "callers": 60, "callees": 40, "tests": 40, "references": 60},
	query.IntentExports:    {"exports": 500, "imports": 420, "callers": 40, "callees": 40, "tests": 40, "references": 60},
	query.IntentReferences: {"references": 500, "callers": 60, "callees": 60, "tests": 60, "imports": 40, "exports": 40},
	query.IntentImpact:     {"callers": 400, "tests": 400, "references": 400, "callees": 160, "imports": 120, "exports": 120},
}

func ProfileFor(plan query.Plan) Profile {
	primary := plan.PrimaryIntent
	if primary == "" {
		primary = query.IntentDefinition
	}
	profile := Profile{
		Name:                 string(primary),
		QualifiedExact:       120,
		SymbolExact:          100,
		Prefix:               70,
		NaturalPrefix:        24,
		LexicalMax:           40,
		PathMax:              20,
		ChangedFile:          15,
		TestMatch:            15,
		NonTestPenalty:       -60,
		DocumentationPenalty: -40,
		LargeSpanPenalty:     -10,
		RelationWeights:      copyRelationWeights(relationWeightsByIntent[primary]),
	}
	return profile
}

func copyRelationWeights(source map[string]float64) map[string]float64 {
	result := make(map[string]float64, len(source))
	for relation, weight := range source {
		result[relation] = weight
	}
	return result
}
