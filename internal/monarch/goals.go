package monarch

import (
	"context"

	"github.com/thedavidweng/monarchmoney-cli/internal/graphql"
	"github.com/thedavidweng/monarchmoney-cli/internal/money"
	"github.com/thedavidweng/monarchmoney-cli/queries"
)

var GetGoalsQuery = queries.Get("goals/list.graphql")
var GetSavingsGoalBudgetsQuery = queries.Get("goals/budgets.graphql")

type Goal struct {
	ID                                    string  `json:"id"`
	Type                                  string  `json:"type"`
	Name                                  string  `json:"name"`
	Status                                string  `json:"status"`
	Progress                              float64 `json:"progress"`
	CurrentBalance                        float64 `json:"current_balance"`
	TargetDate                            string  `json:"target_date"`
	TargetAmount                          float64 `json:"target_amount"`
	PlannedMonthlyContribution            float64 `json:"planned_monthly_contribution"`
	CurrentMonthPlannedContributionAmount float64 `json:"current_month_planned_contribution_amount"`
	SpendingTotal                         float64 `json:"spending_total"`
	NetContribution                       float64 `json:"net_contribution"`
	EstimatedMonthsUntilCompletion        int     `json:"estimated_months_until_completion"`
	ForecastedCompletionDate              string  `json:"forecasted_completion_date"`
	IsSinkingFund                         bool    `json:"is_sinking_fund"`
	Priority                              int     `json:"priority"`
}

type SavingsGoalBudget struct {
	ID         string  `json:"id"`
	GoalID     string  `json:"goal_id"`
	GoalName   string  `json:"goal_name"`
	GoalType   string  `json:"goal_type"`
	GoalStatus string  `json:"goal_status"`
	Month      string  `json:"month"`
	Planned    float64 `json:"planned"`
	Actual     float64 `json:"actual"`
	Remaining  float64 `json:"remaining"`
}

func (s *Service) ListGoals(ctx context.Context) ([]Goal, error) {
	var resp struct {
		SavingsGoals []struct {
			ID                                    string  `json:"id"`
			Type                                  string  `json:"type"`
			Name                                  string  `json:"name"`
			Status                                string  `json:"status"`
			Progress                              float64 `json:"progress"`
			CurrentBalance                        float64 `json:"currentBalance"`
			TargetDate                            string  `json:"targetDate"`
			TargetAmount                          float64 `json:"targetAmount"`
			PlannedMonthlyContribution            float64 `json:"plannedMonthlyContribution"`
			CurrentMonthPlannedContributionAmount float64 `json:"currentMonthPlannedContributionAmount"`
			SpendingTotal                         float64 `json:"spendingTotal"`
			NetContribution                       float64 `json:"netContribution"`
			EstimatedMonthsUntilCompletion        int     `json:"estimatedMonthsUntilCompletion"`
			ForecastedCompletionDate              string  `json:"forecastedCompletionDate"`
			IsSinkingFund                         bool    `json:"isSinkingFund"`
			Priority                              int     `json:"priority"`
		} `json:"savingsGoals"`
	}

	err := s.Client.Do(ctx, &graphql.Request{
		OperationName: "Common_SavingsGoals",
		Query:         GetGoalsQuery,
	}, &resp)
	if err != nil {
		return nil, err
	}

	goals := make([]Goal, len(resp.SavingsGoals))
	for i := range resp.SavingsGoals {
		g := &resp.SavingsGoals[i]
		goals[i] = Goal{
			ID:                                    g.ID,
			Type:                                  g.Type,
			Name:                                  g.Name,
			Status:                                g.Status,
			Progress:                              g.Progress,
			CurrentBalance:                        money.Round2(g.CurrentBalance),
			TargetDate:                            g.TargetDate,
			TargetAmount:                          money.Round2(g.TargetAmount),
			PlannedMonthlyContribution:            money.Round2(g.PlannedMonthlyContribution),
			CurrentMonthPlannedContributionAmount: money.Round2(g.CurrentMonthPlannedContributionAmount),
			SpendingTotal:                         money.Round2(g.SpendingTotal),
			NetContribution:                       money.Round2(g.NetContribution),
			EstimatedMonthsUntilCompletion:        g.EstimatedMonthsUntilCompletion,
			ForecastedCompletionDate:              g.ForecastedCompletionDate,
			IsSinkingFund:                         g.IsSinkingFund,
			Priority:                              g.Priority,
		}
	}
	return goals, nil
}

func (s *Service) ListSavingsGoalBudgets(ctx context.Context, startDate, endDate string) ([]SavingsGoalBudget, error) {
	var resp struct {
		SavingsGoalMonthlyBudgetAmounts []struct {
			ID          string `json:"id"`
			SavingsGoal struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Type   string `json:"type"`
				Status string `json:"status"`
			} `json:"savingsGoal"`
			MonthlyAmounts []struct {
				Month           string  `json:"month"`
				PlannedAmount   float64 `json:"plannedAmount"`
				ActualAmount    float64 `json:"actualAmount"`
				RemainingAmount float64 `json:"remainingAmount"`
			} `json:"monthlyAmounts"`
		} `json:"savingsGoalMonthlyBudgetAmounts"`
	}

	err := s.Client.Do(ctx, &graphql.Request{
		OperationName: "GetSavingsGoals",
		Query:         GetSavingsGoalBudgetsQuery,
		Variables: map[string]any{
			"startDate": startDate,
			"endDate":   endDate,
		},
	}, &resp)
	if err != nil {
		return nil, err
	}

	budgets := make([]SavingsGoalBudget, 0)
	for _, item := range resp.SavingsGoalMonthlyBudgetAmounts {
		for _, m := range item.MonthlyAmounts {
			budgets = append(budgets, SavingsGoalBudget{
				ID:         item.ID,
				GoalID:     item.SavingsGoal.ID,
				GoalName:   item.SavingsGoal.Name,
				GoalType:   item.SavingsGoal.Type,
				GoalStatus: item.SavingsGoal.Status,
				Month:      m.Month,
				Planned:    money.Round2(m.PlannedAmount),
				Actual:     money.Round2(m.ActualAmount),
				Remaining:  money.Round2(m.RemainingAmount),
			})
		}
	}
	return budgets, nil
}
