package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/subzone/Agentctl/internal/config"
)

// LoadCommandTools scans dir for tool MD files (type: tool, runtime: shell)
// and builds an executable CommandTool for each. Non-shell tool specs and
// non-tool documents are skipped. A missing directory is not an error.
// Parse/validation failures are collected and returned so callers can surface
// them without aborting the whole load.
func LoadCommandTools(dir string) ([]Tool, []error) {
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}
	var (
		out  []Tool
		errs []error
	)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		doc, err := config.ParseFile(p)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
			continue
		}
		spec, ok := doc.Spec.(*config.ToolSpec)
		if !ok {
			continue
		}
		if spec.Runtime != config.RuntimeShell {
			continue
		}
		desc := spec.Description
		if body := strings.TrimSpace(doc.Body); body != "" {
			// The markdown body augments the frontmatter description with
			// usage notes the model sees alongside the tool.
			if desc != "" {
				desc += "\n\n"
			}
			desc += body
		}
		out = append(out, NewCommandTool(
			spec.Name, desc, spec.Command, spec.Env,
			spec.ParametersJSON(), time.Duration(spec.TimeoutSec)*time.Second,
		))
	}
	return out, errs
}
