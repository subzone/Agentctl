package desktop

import (
	"os"

	"github.com/subzone/Agentctl/internal/atfile"
)

// FileCandidate is a workspace file for @-mention autocomplete.
type FileCandidate = atfile.Candidate

// SearchAtFiles finds files under the working directory matching query.
func (a *App) SearchAtFiles(query string) []FileCandidate {
	root, err := os.Getwd()
	if err != nil {
		return nil
	}
	return atfile.Search(root, query, 12)
}

// ExpandAtFiles inlines @file references in a message (same as CLI chat).
func (a *App) ExpandAtFiles(input string) (string, []string) {
	return atfile.Expand(input)
}
