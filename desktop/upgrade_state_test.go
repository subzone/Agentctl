package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClearPendingUpdateIfInstalled(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	savePendingUpdate("0.8.3", "/tmp/AgentCTL_0.8.3_macos.zip")
	if loadPendingUpdate() == nil {
		t.Fatal("expected pending update")
	}

	clearPendingUpdateIfInstalled("0.8.3", "0.8.3")
	if loadPendingUpdate() != nil {
		t.Fatal("expected pending cleared when current matches latest")
	}

	savePendingUpdate("0.8.4", "/tmp/AgentCTL_0.8.4_macos.zip")
	clearPendingUpdateIfInstalled("0.8.3", "0.8.4")
	if loadPendingUpdate() == nil {
		t.Fatal("expected pending to remain when still behind latest")
	}
}

func TestPendingUpdatePath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	p := pendingUpdatePath()
	if p == "" {
		t.Fatal("expected path")
	}
	if filepath.Base(filepath.Dir(p)) != "m" {
		t.Fatalf("unexpected path: %s", p)
	}

	savePendingUpdate("1.0.0", "/downloads/update.zip")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("expected written state")
	}
}
