package search

import "github.com/focalspan/focalspan/internal/model"

// Unit fixtures already return their complete candidate sets; SQL scoping is
// exercised by real-store/service integration tests. Final filtering remains tested here.
func (f fakeStore) ScopedCandidates(model.CandidateScope) model.CandidateReader         { return f }
func (f relationFakeStore) ScopedCandidates(model.CandidateScope) model.CandidateReader { return f }
func (f *retrievalRecordingStore) ScopedCandidates(model.CandidateScope) model.CandidateReader {
	return f
}
