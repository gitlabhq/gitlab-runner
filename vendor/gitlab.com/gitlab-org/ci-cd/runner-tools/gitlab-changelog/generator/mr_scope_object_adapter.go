package generator

import (
	"fmt"

	"gitlab.com/gitlab-org/ci-cd/runner-tools/gitlab-changelog/gitlab"
)

type MRScopeObjectAdapter struct {
	mr            gitlab.MergeRequest
	includeAuthor bool
}

func NewMRScopeObjectAdapter(mr gitlab.MergeRequest) *MRScopeObjectAdapter {
	return &MRScopeObjectAdapter{
		mr:            mr,
		includeAuthor: false,
	}
}

func (a *MRScopeObjectAdapter) IncludeAuthor() {
	a.includeAuthor = true
}

func (a *MRScopeObjectAdapter) Labels() []string {
	return a.mr.Labels
}

func (a *MRScopeObjectAdapter) Entry() string {
	entry := fmt.Sprintf("%s !%d", a.mr.Title, a.mr.IID)
	if a.includeAuthor {
		entry = fmt.Sprintf("%s (%s)", entry, a.mr.Author())
	}

	return entry
}
