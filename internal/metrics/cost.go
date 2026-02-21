package metrics

import (
	"github.com/JuanVilla424/teamoon/internal/config"
)

type CostSummary struct {
	PlanCost      float64 `json:"plan_cost"`
	SessionsToday int     `json:"sessions_today"`
	SessionsWeek  int     `json:"sessions_week"`
	SessionsMonth int     `json:"sessions_month"`
	OutputToday   int     `json:"output_today"`
	OutputWeek    int     `json:"output_week"`
	OutputMonth   int     `json:"output_month"`
}

func CalculateCost(today, week, month TokenSummary, cfg config.Config) CostSummary {
	return CostSummary{
		PlanCost:       cfg.BudgetMonthly,
		SessionsToday:  today.SessionCount,
		SessionsWeek:   week.SessionCount,
		SessionsMonth:  month.SessionCount,
		OutputToday:    today.Output,
		OutputWeek:     week.Output,
		OutputMonth:    month.Output,
	}
}
