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
	if language == "php" && (relation.Kind == "calls" || relation.Kind == "tests") && keys != nil {
		target := strings.TrimLeft(relation.UnresolvedTo, "\\")
		keys["php_absolute"] = normalizeLookupKey(target)
		if at := strings.LastIndex(target, "::"); at >= 0 {
			target = target[at+2:]
		}
		if at := strings.LastIndex(target, "->"); at >= 0 {
			target = target[at+2:]
		}
		if at := strings.LastIndexByte(target, '\\'); at >= 0 {
			target = target[at+1:]
		}
		keys["php_name"] = normalizeLookupKey(target)
	}
	if language != "csharp" || (relation.Kind != "calls" && relation.Kind != "tests") || keys == nil {
		return keys
	}
	if dot := strings.LastIndexByte(relation.UnresolvedTo, '.'); dot >= 0 && dot+1 < len(relation.UnresolvedTo) {
		keys["csharp_member"] = normalizeLookupKey(relation.UnresolvedTo[dot+1:])
	}
	return keys
}
