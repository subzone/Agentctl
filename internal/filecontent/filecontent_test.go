package filecontent_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/subzone/Agentctl/internal/filecontent"
)

func TestReadTextFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("hello world\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := filecontent.Read(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if out != "hello world" {
		t.Fatalf("got %q", out)
	}
}

func TestReadPDFExtension(t *testing.T) {
	// Invalid PDF bytes still route through the PDF parser.
	_, err := filecontent.ReadBytes("doc.pdf", []byte("%PDF-1.4\nnot-a-real-pdf"), 0)
	if err == nil {
		t.Fatal("expected parse error for malformed PDF")
	}
	if !strings.Contains(err.Error(), "PDF") && !strings.Contains(err.Error(), "parse") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadBinaryRejected(t *testing.T) {
	_, err := filecontent.ReadBytes("data.bin", []byte{0, 1, 2, 3}, 0)
	if err == nil {
		t.Fatal("expected error for binary")
	}
}
