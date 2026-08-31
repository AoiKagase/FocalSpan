package benchmark

type CaseProposal struct {
	Case              Case     `json:"case"`
	CandidatePaths    []string `json:"candidate_paths"`
	UnavailableAtBase []string `json:"unavailable_at_base"`
}

func BuildCaseProposal(repository, query string, changes ChangeSet) CaseProposal {
	p := CaseProposal{Case: Case{ID: "proposal", Repository: repository, BaseRef: changes.BaseCommit, TargetRef: changes.TargetCommit, Query: query, Budgets: []int{1024, 2048, 4096}}}
	for _, f := range changes.Files {
		if f.Status == "add" {
			p.UnavailableAtBase = append(p.UnavailableAtBase, f.NewPath)
		} else {
			p.CandidatePaths = append(p.CandidatePaths, f.NewPath)
		}
	}
	return p
}
