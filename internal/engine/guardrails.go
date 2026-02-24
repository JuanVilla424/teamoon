package engine

import (
	"fmt"
	"sync"
	"time"

	"github.com/JuanVilla424/teamoon/internal/metrics"
)

const cacheTTL = 60 * time.Second

type guardrailSnapshot struct {
	usage   metrics.ClaudeUsage
	fetched time.Time
}

var (
	grMu    sync.Mutex
	grCache guardrailSnapshot
)

func refreshCache() guardrailSnapshot {
	grMu.Lock()
	defer grMu.Unlock()

	if time.Since(grCache.fetched) < cacheTTL {
		return grCache
	}

	grCache = guardrailSnapshot{
		usage:   metrics.GetUsage(),
		fetched: time.Now(),
	}
	return grCache
}

// CheckGuardrails returns a non-empty reason string if the engine should pause.
// Thresholds are driven by Claude's own usage percentages (session + weekly).
// Returns "" if it's safe to proceed.
func CheckGuardrails() string {
	snap := refreshCache()

	if snap.usage.WeekAll.Utilization >= 90 {
		return fmt.Sprintf("Claude weekly usage at %.0f%% — pausing", snap.usage.WeekAll.Utilization)
	}

	if snap.usage.Session.Utilization >= 90 {
		return fmt.Sprintf("Claude session usage at %.0f%% — pausing", snap.usage.Session.Utilization)
	}

	return ""
}
