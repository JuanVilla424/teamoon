package engine

import (
	"fmt"
	"sync"
	"time"

	"github.com/JuanVilla424/teamoon/internal/config"
	"github.com/JuanVilla424/teamoon/internal/metrics"
)

const cacheTTL = 60 * time.Second

type guardrailSnapshot struct {
	usage   metrics.ClaudeUsage
	cost    metrics.CostSummary
	fetched time.Time
}

var (
	grMu    sync.Mutex
	grCache guardrailSnapshot
)

func refreshCache(cfg config.Config) guardrailSnapshot {
	grMu.Lock()
	defer grMu.Unlock()

	if time.Since(grCache.fetched) < cacheTTL {
		return grCache
	}

	usage := metrics.GetUsage()

	today, week, month, _ := metrics.ScanTokens(cfg.ClaudeDir)
	cost := metrics.CalculateCost(today, week, month, cfg)

	grCache = guardrailSnapshot{
		usage:   usage,
		cost:    cost,
		fetched: time.Now(),
	}
	return grCache
}

// CheckGuardrails returns a non-empty reason string if the engine should pause.
// Returns "" if it's safe to proceed.
func CheckGuardrails(cfg config.Config) string {
	snap := refreshCache(cfg)

	// Claude weekly usage (all models)
	if snap.usage.WeekAll.Utilization >= 90 {
		return fmt.Sprintf("Claude weekly usage at %.0f%% — pausing", snap.usage.WeekAll.Utilization)
	}

	// Claude session usage
	if snap.usage.Session.Utilization >= 90 {
		return fmt.Sprintf("Claude session usage at %.0f%% — pausing", snap.usage.Session.Utilization)
	}

	// Monthly budget
	if cfg.BudgetMonthly > 0 && snap.cost.CostMonth >= cfg.BudgetMonthly*0.95 {
		return fmt.Sprintf("Monthly budget at 95%% ($%.2f / $%.2f) — pausing", snap.cost.CostMonth, cfg.BudgetMonthly)
	}

	return ""
}
