package evidence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/focalspan/focalspan/internal/budget"
)

const SchemaContextV2 = "focalspan.context.v2"

type contextV2Wire struct {
	Schema       string              `json:"schema"`
	Revision     string              `json:"r,omitempty"`
	Intent       string              `json:"i,omitempty"`
	Mode         Mode                `json:"m"`
	Budget       []json.RawMessage   `json:"b"`
	Paths        []string            `json:"p,omitempty"`
	Evidence     [][]json.RawMessage `json:"e"`
	Relations    [][]json.RawMessage `json:"x,omitempty"`
	Limitations  []string            `json:"l,omitempty"`
	Next         [][]json.RawMessage `json:"n,omitempty"`
	SkippedKnown int                 `json:"k,omitempty"`
}

// EncodeContextV2 converts a valid v1 packet into the negotiated compact wire
// representation. Budget.Used is re-settled against the v2 representation and
// its unchanged text summary.
func EncodeContextV2(packet Packet, estimator budget.TokenEstimator) (json.RawMessage, error) {
	if err := Validate(packet); err != nil {
		return nil, fmt.Errorf("encode context v2: %w", err)
	}
	if estimator == nil {
		estimator = budget.NewEstimator()
	}
	wire, err := makeContextV2Wire(packet)
	if err != nil {
		return nil, err
	}
	for iteration := 0; iteration < 6; iteration++ {
		raw, err := json.Marshal(wire)
		if err != nil {
			return nil, fmt.Errorf("encode context v2: %w", err)
		}
		used := estimator.Estimate(string(raw) + "\n" + summaryWithUsed(packet, wireBudgetUsed(wire.Budget)))
		if used == wireBudgetUsed(wire.Budget) {
			if used > packet.Budget.Limit {
				return nil, fmt.Errorf("encode context v2: wire budget %d exceeds limit %d", used, packet.Budget.Limit)
			}
			return raw, nil
		}
		wire.Budget[1] = rawJSON(used)
	}
	raw, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode context v2: %w", err)
	}
	used := estimator.Estimate(string(raw) + "\n" + summaryWithUsed(packet, wireBudgetUsed(wire.Budget)))
	wire.Budget[1] = rawJSON(used)
	raw, err = json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode context v2: %w", err)
	}
	if used > packet.Budget.Limit {
		return nil, fmt.Errorf("encode context v2: wire budget %d exceeds limit %d", used, packet.Budget.Limit)
	}
	return raw, nil
}

// PreferContextV2 returns the compact encoding and its canonical decoded
// packet only when the v2 model-visible wire estimate is strictly below v1.
func PreferContextV2(packet Packet, estimator budget.TokenEstimator) (json.RawMessage, Packet, bool, error) {
	if estimator == nil {
		estimator = budget.NewEstimator()
	}
	raw, err := EncodeContextV2(packet, estimator)
	if err != nil {
		return nil, Packet{}, false, err
	}
	decoded, err := DecodeContextV2(raw)
	if err != nil {
		return nil, Packet{}, false, err
	}
	if decoded.Budget.Used >= MeasureModelVisible(packet, estimator) {
		return nil, packet, false, nil
	}
	return raw, decoded, true, nil
}

func makeContextV2Wire(packet Packet) (contextV2Wire, error) {
	wire := contextV2Wire{
		Schema:       SchemaContextV2,
		Revision:     packet.Revision,
		Intent:       packet.Intent,
		Mode:         packet.Mode,
		Budget:       []json.RawMessage{rawJSON(packet.Budget.Limit), rawJSON(0), rawJSON(packet.Budget.Truncated), rawJSON(packet.Budget.Omitted)},
		Evidence:     make([][]json.RawMessage, 0, len(packet.Evidence)),
		Limitations:  append([]string(nil), packet.Limitations...),
		SkippedKnown: packet.SkippedKnown,
	}
	pathIndexes := make(map[string]int, len(packet.Evidence))
	idIndexes := make(map[string]int, len(packet.Evidence))
	for index, item := range packet.Evidence {
		pathIndex, exists := pathIndexes[item.Location.Path]
		if !exists {
			pathIndex = len(wire.Paths)
			pathIndexes[item.Location.Path] = pathIndex
			wire.Paths = append(wire.Paths, item.Location.Path)
		}
		idIndexes[item.ID] = index
		payload, err := contextV2Payload(item)
		if err != nil {
			return contextV2Wire{}, err
		}
		attributes := map[string]any{}
		if item.Language != "" {
			attributes["a"] = item.Language
		}
		if item.Kind != "" {
			attributes["k"] = item.Kind
		}
		if item.Symbol != "" {
			attributes["s"] = item.Symbol
		}
		if len(item.Why) > 0 {
			attributes["w"] = item.Why
		}
		wire.Evidence = append(wire.Evidence, rawRow(item.Handle, item.Role, pathIndex, item.Location.Lines[0], item.Location.Lines[1], item.Fidelity, payload, attributes))
	}
	for _, edge := range packet.Relations {
		from, fromOK := idIndexes[edge.From]
		to, toOK := idIndexes[edge.To]
		if !fromOK || !toOK {
			return contextV2Wire{}, fmt.Errorf("encode context v2: relation references missing ID")
		}
		wire.Relations = append(wire.Relations, rawRow(from, to, edge.Kind, edge.Certainty))
	}
	for _, action := range packet.Next {
		wire.Next = append(wire.Next, rawRow(action.Handle, action.Relation, action.Reason))
	}
	return wire, nil
}

func contextV2Payload(item Item) (any, error) {
	switch item.Fidelity {
	case FidelityVerbatim:
		return item.Source, nil
	case FidelitySignature:
		return item.Signature, nil
	case FidelitySynthetic:
		return item.Outline, nil
	case FidelityExcerpt:
		segments := make([][]json.RawMessage, 0, len(item.Segments))
		for _, segment := range item.Segments {
			if segment.Text == "" {
				segments = append(segments, rawRow(segment.Kind, segment.Lines[0], segment.Lines[1]))
			} else {
				segments = append(segments, rawRow(segment.Kind, segment.Lines[0], segment.Lines[1], segment.Text))
			}
		}
		return segments, nil
	default:
		return nil, fmt.Errorf("encode context v2: unsupported fidelity %q", item.Fidelity)
	}
}

func rawRow(values ...any) []json.RawMessage {
	row := make([]json.RawMessage, len(values))
	for index, value := range values {
		row[index] = rawJSON(value)
	}
	return row
}

func rawJSON(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}

func wireBudgetUsed(row []json.RawMessage) int {
	if len(row) != 4 {
		return 0
	}
	value, _ := rawInt(row[1])
	return value
}

func summaryWithUsed(packet Packet, used int) string {
	copy := packet
	copy.Budget.Used = used
	return Summary(copy)
}

// DecodeContextV2 strictly decodes the compact representation into the
// canonical v1 packet and validates all public Evidence Packet invariants.
func DecodeContextV2(data []byte) (Packet, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var wire contextV2Wire
	if err := decoder.Decode(&wire); err != nil {
		return Packet{}, fmt.Errorf("decode context v2: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Packet{}, err
	}
	if wire.Schema != SchemaContextV2 {
		return Packet{}, fmt.Errorf("decode context v2: invalid schema %q", wire.Schema)
	}
	if wire.Evidence == nil {
		return Packet{}, fmt.Errorf("decode context v2: evidence table is required")
	}
	if len(wire.Budget) != 4 {
		return Packet{}, fmt.Errorf("decode context v2: budget row must have 4 fields")
	}
	limit, err := rawInt(wire.Budget[0])
	if err != nil {
		return Packet{}, fmt.Errorf("decode context v2: budget limit: %w", err)
	}
	used, err := rawInt(wire.Budget[1])
	if err != nil {
		return Packet{}, fmt.Errorf("decode context v2: budget used: %w", err)
	}
	truncated, err := rawBool(wire.Budget[2])
	if err != nil {
		return Packet{}, fmt.Errorf("decode context v2: budget truncated: %w", err)
	}
	omitted, err := rawInt(wire.Budget[3])
	if err != nil {
		return Packet{}, fmt.Errorf("decode context v2: budget omitted: %w", err)
	}
	seenPaths := make(map[string]struct{}, len(wire.Paths))
	for _, path := range wire.Paths {
		if _, duplicate := seenPaths[path]; duplicate {
			return Packet{}, fmt.Errorf("decode context v2: duplicate path table entry %q", path)
		}
		seenPaths[path] = struct{}{}
	}
	packet := Packet{
		Schema:       SchemaContextV1,
		Revision:     wire.Revision,
		Intent:       wire.Intent,
		Mode:         wire.Mode,
		Budget:       Budget{Limit: limit, Used: used, Truncated: truncated, Omitted: omitted},
		Evidence:     make([]Item, 0, len(wire.Evidence)),
		Limitations:  append([]string(nil), wire.Limitations...),
		SkippedKnown: wire.SkippedKnown,
	}
	for index, row := range wire.Evidence {
		item, err := decodeContextV2Item(index, row, wire.Paths)
		if err != nil {
			return Packet{}, err
		}
		packet.Evidence = append(packet.Evidence, item)
	}
	for _, row := range wire.Relations {
		if len(row) != 4 {
			return Packet{}, fmt.Errorf("decode context v2: relation row must have 4 fields")
		}
		from, err := rawInt(row[0])
		if err != nil || from < 0 || from >= len(packet.Evidence) {
			return Packet{}, fmt.Errorf("decode context v2: relation from index is invalid")
		}
		to, err := rawInt(row[1])
		if err != nil || to < 0 || to >= len(packet.Evidence) {
			return Packet{}, fmt.Errorf("decode context v2: relation to index is invalid")
		}
		kind, err := rawString(row[2])
		if err != nil {
			return Packet{}, fmt.Errorf("decode context v2: relation kind: %w", err)
		}
		certainty, err := rawString(row[3])
		if err != nil {
			return Packet{}, fmt.Errorf("decode context v2: relation certainty: %w", err)
		}
		packet.Relations = append(packet.Relations, Edge{From: "e" + strconv.Itoa(from+1), To: "e" + strconv.Itoa(to+1), Kind: kind, Certainty: Certainty(certainty)})
	}
	for _, row := range wire.Next {
		if len(row) != 3 {
			return Packet{}, fmt.Errorf("decode context v2: next row must have 3 fields")
		}
		handle, err := rawString(row[0])
		if err != nil {
			return Packet{}, fmt.Errorf("decode context v2: next handle: %w", err)
		}
		relation, err := rawString(row[1])
		if err != nil {
			return Packet{}, fmt.Errorf("decode context v2: next relation: %w", err)
		}
		reason, err := rawString(row[2])
		if err != nil {
			return Packet{}, fmt.Errorf("decode context v2: next reason: %w", err)
		}
		packet.Next = append(packet.Next, NextAction{Handle: handle, Relation: relation, Reason: reason})
	}
	if err := Validate(packet); err != nil {
		return Packet{}, fmt.Errorf("decode context v2: %w", err)
	}
	return packet, nil
}

func decodeContextV2Item(index int, row []json.RawMessage, paths []string) (Item, error) {
	if len(row) != 8 {
		return Item{}, fmt.Errorf("decode context v2: evidence row %d must have 8 fields", index)
	}
	handle, err := rawString(row[0])
	if err != nil {
		return Item{}, fmt.Errorf("decode context v2: evidence row %d handle: %w", index, err)
	}
	role, err := rawString(row[1])
	if err != nil {
		return Item{}, fmt.Errorf("decode context v2: evidence row %d role: %w", index, err)
	}
	pathIndex, err := rawInt(row[2])
	if err != nil || pathIndex < 0 || pathIndex >= len(paths) {
		return Item{}, fmt.Errorf("decode context v2: evidence row %d path index is invalid", index)
	}
	start, err := rawInt(row[3])
	if err != nil {
		return Item{}, fmt.Errorf("decode context v2: evidence row %d start line: %w", index, err)
	}
	end, err := rawInt(row[4])
	if err != nil {
		return Item{}, fmt.Errorf("decode context v2: evidence row %d end line: %w", index, err)
	}
	fidelityValue, err := rawString(row[5])
	if err != nil {
		return Item{}, fmt.Errorf("decode context v2: evidence row %d fidelity: %w", index, err)
	}
	item := Item{ID: "e" + strconv.Itoa(index+1), Handle: handle, Role: Role(role), Location: Location{Path: paths[pathIndex], Lines: [2]int{start, end}}, Fidelity: Fidelity(fidelityValue)}
	if err := decodeContextV2Payload(&item, row[6]); err != nil {
		return Item{}, fmt.Errorf("decode context v2: evidence row %d payload: %w", index, err)
	}
	var attributes map[string]json.RawMessage
	if err := json.Unmarshal(row[7], &attributes); err != nil || attributes == nil {
		return Item{}, fmt.Errorf("decode context v2: evidence row %d attributes must be an object", index)
	}
	for key, value := range attributes {
		switch key {
		case "a":
			item.Language, err = rawString(value)
		case "k":
			item.Kind, err = rawString(value)
		case "s":
			item.Symbol, err = rawString(value)
		case "w":
			err = json.Unmarshal(value, &item.Why)
		default:
			return Item{}, fmt.Errorf("decode context v2: evidence row %d has unknown attribute %q", index, key)
		}
		if err != nil {
			return Item{}, fmt.Errorf("decode context v2: evidence row %d attribute %q: %w", index, key, err)
		}
	}
	return item, nil
}

func decodeContextV2Payload(item *Item, raw json.RawMessage) error {
	switch item.Fidelity {
	case FidelityVerbatim:
		return json.Unmarshal(raw, &item.Source)
	case FidelitySignature:
		return json.Unmarshal(raw, &item.Signature)
	case FidelitySynthetic:
		return json.Unmarshal(raw, &item.Outline)
	case FidelityExcerpt:
		var rows [][]json.RawMessage
		if err := json.Unmarshal(raw, &rows); err != nil {
			return err
		}
		for _, row := range rows {
			if len(row) != 3 && len(row) != 4 {
				return fmt.Errorf("segment row must have 3 or 4 fields")
			}
			kind, err := rawString(row[0])
			if err != nil {
				return err
			}
			start, err := rawInt(row[1])
			if err != nil {
				return err
			}
			end, err := rawInt(row[2])
			if err != nil {
				return err
			}
			segment := Segment{Kind: kind, Lines: [2]int{start, end}}
			if len(row) == 4 {
				if segment.Text, err = rawString(row[3]); err != nil {
					return err
				}
			}
			item.Segments = append(item.Segments, segment)
		}
		return nil
	default:
		return fmt.Errorf("unsupported fidelity %q", item.Fidelity)
	}
}

func rawString(raw json.RawMessage) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	return value, nil
}

func rawInt(raw json.RawMessage) (int, error) {
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, err
	}
	return value, nil
}

func rawBool(raw json.RawMessage) (bool, error) {
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, err
	}
	return value, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("decode context v2: trailing JSON value")
	}
	return fmt.Errorf("decode context v2: %w", err)
}
