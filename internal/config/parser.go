package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// fenceLine is the YAML frontmatter delimiter.
const fenceLine = "---"

// ErrMissingFrontmatter is returned when a file has no YAML frontmatter block.
var ErrMissingFrontmatter = errors.New("missing YAML frontmatter")

// ErrUnknownType is returned when frontmatter declares an unknown type.
var ErrUnknownType = errors.New("unknown document type")

// ParseFile reads and parses a single MD file at path.
func ParseFile(path string) (*Document, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc, err := Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	doc.Path = path
	return doc, nil
}

// Parse parses the bytes of a single MD file with YAML frontmatter.
func Parse(raw []byte) (*Document, error) {
	fm, body, err := splitFrontmatter(raw)
	if err != nil {
		return nil, err
	}

	// First pass: read just enough to discover the document type.
	var head Meta
	if err := yaml.Unmarshal(fm, &head); err != nil {
		return nil, fmt.Errorf("frontmatter yaml: %w", err)
	}

	doc := &Document{Body: strings.TrimSpace(string(body))}

	// Second pass: decode into the type-specific spec with strict mode so
	// unknown fields surface as errors.
	dec := yaml.NewDecoder(bytes.NewReader(fm))
	dec.KnownFields(true)

	switch head.Type {
	case TypeAgent:
		spec := &AgentSpec{}
		if err := dec.Decode(spec); err != nil {
			return nil, fmt.Errorf("agent frontmatter: %w", err)
		}
		doc.Spec = spec
	case TypeSkill:
		spec := &SkillSpec{}
		if err := dec.Decode(spec); err != nil {
			return nil, fmt.Errorf("skill frontmatter: %w", err)
		}
		doc.Spec = spec
	case TypeTool:
		spec := &ToolSpec{}
		if err := dec.Decode(spec); err != nil {
			return nil, fmt.Errorf("tool frontmatter: %w", err)
		}
		doc.Spec = spec
	case TypeMCPServer:
		spec := &MCPServerSpec{}
		if err := dec.Decode(spec); err != nil {
			return nil, fmt.Errorf("mcp_server frontmatter: %w", err)
		}
		doc.Spec = spec
	case "":
		return nil, errors.New("frontmatter missing required field: type")
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownType, head.Type)
	}

	return doc, nil
}

// splitFrontmatter extracts the YAML block between the leading and trailing
// `---` fences and returns it alongside the remaining body.
func splitFrontmatter(raw []byte) (frontmatter, body []byte, err error) {
	// Normalize CRLF without copying when possible.
	src := raw
	if bytes.Contains(src, []byte("\r\n")) {
		src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	}

	// Allow a leading BOM and any leading blank lines, but require the
	// opening fence to be the first non-empty line.
	src = bytes.TrimPrefix(src, []byte{0xEF, 0xBB, 0xBF})
	trimmed := bytes.TrimLeft(src, "\n")
	if !bytes.HasPrefix(trimmed, []byte(fenceLine+"\n")) &&
		!bytes.Equal(trimmed, []byte(fenceLine)) {
		return nil, nil, ErrMissingFrontmatter
	}

	// Skip the opening fence.
	rest := trimmed[len(fenceLine):]
	rest = bytes.TrimPrefix(rest, []byte("\n"))

	// Find the closing fence on its own line.
	closing := []byte("\n" + fenceLine)
	idx := bytes.Index(rest, closing)
	if idx < 0 {
		// Allow a fence at very start of remaining (empty frontmatter).
		if bytes.HasPrefix(rest, []byte(fenceLine)) {
			idx = 0
		} else {
			return nil, nil, fmt.Errorf("%w: closing fence not found", ErrMissingFrontmatter)
		}
	}

	frontmatter = rest[:idx]
	bodyStart := idx + len(closing)
	if bodyStart > len(rest) {
		bodyStart = len(rest)
	}
	body = rest[bodyStart:]
	body = bytes.TrimPrefix(body, []byte("\n"))
	return frontmatter, body, nil
}

// LoadDir walks dir recursively and parses every .md file. Parse errors are
// returned as a joined error so callers see all problems at once.
func LoadDir(dir string) ([]*Document, error) {
	var docs []*Document
	var errs []error

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		doc, err := ParseFile(path)
		if err != nil {
			errs = append(errs, err)
			return nil
		}
		docs = append(docs, doc)
		return nil
	})
	if err != nil {
		errs = append(errs, err)
	}
	return docs, errors.Join(errs...)
}
