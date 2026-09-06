package metrics

import (
	"testing"
)

func TestHostCollector_Collect(t *testing.T) {
	collector := NewHostCollector("", "", "")
	metrics, err := collector.Collect()
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if metrics.DiskTotal == 0 {
		t.Errorf("expected non-zero DiskTotal, got 0")
	}

	if metrics.DiskUsed > metrics.DiskTotal {
		t.Errorf("DiskUsed (%d) cannot exceed DiskTotal (%d)", metrics.DiskUsed, metrics.DiskTotal)
	}
}
