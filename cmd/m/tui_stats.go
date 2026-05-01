package main

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

// sysStats is the snapshot rendered in the header. GPU is intentionally
// "n/a" in this release — Apple Silicon has no clean public API and
// shelling out to powermetrics needs sudo. Linux NVIDIA support via
// nvidia-smi is a planned follow-up.
type sysStats struct {
	CPU  string
	RAM  string
	GPU  string
	Disk string
}

// blankStats is what the header shows before the first sample lands.
func blankStats() sysStats {
	return sysStats{CPU: "  -", RAM: "  -", GPU: "n/a", Disk: "  -"}
}

// statsSampleCmd produces a fresh sysStats. CPU.Percent with a 0
// duration returns the average since the last call (or boot time on
// first call), so consecutive samples reflect the gap between them
// rather than blocking for a measurement window.
func statsSampleCmd() tea.Cmd {
	return func() tea.Msg {
		out := blankStats()

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		if c, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(c) > 0 {
			out.CPU = fmt.Sprintf("%3.0f%%", c[0])
		}
		if v, err := mem.VirtualMemoryWithContext(ctx); err == nil {
			out.RAM = fmt.Sprintf("%3.0f%%", v.UsedPercent)
		}
		if u, err := disk.UsageWithContext(ctx, "/"); err == nil {
			out.Disk = fmt.Sprintf("%3.0f%%", u.UsedPercent)
		}
		return statsMsg(out)
	}
}

// statsTickCmd waits one second and emits a tick; the model handler
// composes this with statsSampleCmd to refresh on a 1 Hz cadence.
func statsTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return statsTickMsg(t)
	})
}

type statsMsg sysStats
type statsTickMsg time.Time
