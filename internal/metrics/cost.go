package metrics

import (
	"github.com/JuanVilla424/teamoon/internal/config"
)

// Claude API pricing per million tokens (Feb 2026)
type modelPricing struct {
	Input      float64
	Output     float64
	CacheRead  float64
	CacheWrite float64
}

var pricing = map[string]modelPricing{
	"opus":   {Input: 15.0, Output: 75.0, CacheRead: 1.875, CacheWrite: 18.75},
	"sonnet": {Input: 3.0, Output: 15.0, CacheRead: 0.30, CacheWrite: 3.75},
	"haiku":  {Input: 0.80, Output: 4.0, CacheRead: 0.08, CacheWrite: 1.0},
}

func tokenCost(s TokenSummary) float64 {
	if len(s.ByModel) == 0 {
		// Fallback to sonnet pricing if no model info
		p := pricing["sonnet"]
		return (float64(s.Input)*p.Input +
			float64(s.Output)*p.Output +
			float64(s.CacheRead)*p.CacheRead +
			float64(s.CacheCreate)*p.CacheWrite) / 1_000_000.0
	}
	var total float64
	for tier, mt := range s.ByModel {
		p, ok := pricing[tier]
		if !ok {
			p = pricing["sonnet"]
		}
		total += (float64(mt.Input)*p.Input +
			float64(mt.Output)*p.Output +
			float64(mt.CacheRead)*p.CacheRead +
			float64(mt.CacheCreate)*p.CacheWrite) / 1_000_000.0
	}
	return total
}

type CostSummary struct {
	PlanCost      float64 `json:"plan_cost"`
	CostToday     float64 `json:"cost_today"`
	CostWeek      float64 `json:"cost_week"`
	CostMonth     float64 `json:"cost_month"`
	SessionsToday int     `json:"sessions_today"`
	SessionsWeek  int     `json:"sessions_week"`
	SessionsMonth int     `json:"sessions_month"`
	OutputToday   int     `json:"output_today"`
	OutputWeek    int     `json:"output_week"`
	OutputMonth   int     `json:"output_month"`
}

func CalculateCost(today, week, month TokenSummary, cfg config.Config) CostSummary {
	return CostSummary{
		PlanCost:      cfg.BudgetMonthly,
		CostToday:     tokenCost(today),
		CostWeek:      tokenCost(week),
		CostMonth:     tokenCost(month),
		SessionsToday: today.SessionCount,
		SessionsWeek:  week.SessionCount,
		SessionsMonth: month.SessionCount,
		OutputToday:   today.Output,
		OutputWeek:    week.Output,
		OutputMonth:   month.Output,
	}
}
