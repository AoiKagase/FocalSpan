package store

import (
	"github.com/focalspan/focalspan/internal/model"
	"strings"
)

// Qualified C# calls need a member-name dependency key too: this.Save and
// relative type names cannot match a future partial member by full name yet.
// This widens candidate acquisition only; csharpscope still decides resolution.
func sourceRelationLookupKeys(language string, relation model.Relation) map[string]string {
	keys := relationLookupKeys(relation.UnresolvedTo)
	if language != "csharp" || (relation.Kind != "calls" && relation.Kind != "tests") || keys == nil {
		return keys
	}
	if dot := strings.LastIndexByte(relation.UnresolvedTo, '.'); dot >= 0 && dot+1 < len(relation.UnresolvedTo) {
		keys["csharp_member"] = normalizeLookupKey(relation.UnresolvedTo[dot+1:])
	}
	return keys
}
