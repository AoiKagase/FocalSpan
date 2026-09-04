package benchmark

import (
	"encoding/json"

	"github.com/focalspan/focalspan/internal/evidence"
)

func measurePacketForProfile(benchmarkCase Case, profile Profile, tokenBudget int, packet evidence.Packet, deterministic bool, changedPaths []string) (bool, QualityResult, error) {
	measuredPacket, raw, compact, err := packetForProfile(profile, packet)
	if err != nil {
		return false, QualityResult{}, err
	}
	result := MeasurePacket(benchmarkCase, profile.Name, tokenBudget, measuredPacket, deterministic, changedPaths)
	if !compact {
		return false, result, nil
	}
	result.PacketJSONBytes = len(raw)
	result.SummaryBytes = len(evidence.Summary(measuredPacket)) + 1
	result.WireBytes = result.PacketJSONBytes + result.SummaryBytes
	result.EnvelopeMetadataBytes = result.WireBytes - result.SummaryBytes - result.EvidenceContentBytes - result.GuidanceBytes
	return true, result, nil
}

func packetForProfile(profile Profile, packet evidence.Packet) (evidence.Packet, json.RawMessage, bool, error) {
	if profile.ContextEncoding != evidence.SchemaContextV2 {
		return packet, nil, false, nil
	}
	raw, decoded, preferred, err := evidence.PreferContextV2(packet, nil)
	if err != nil {
		return evidence.Packet{}, nil, false, err
	}
	if !preferred {
		return packet, nil, false, nil
	}
	return decoded, raw, true, nil
}
