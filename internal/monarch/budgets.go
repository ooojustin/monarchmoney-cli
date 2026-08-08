package monarch

import (
	"context"
	"fmt"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/graphql"
	"github.com/thedavidweng/monarchmoney-cli/queries"
)

var GetBudgetsQuery = queries.Get("budgets/list.graphql")
var SetBudgetMutation = queries.Get("budgets/set.graphql")
var ResetBudgetMutation = queries.Get("budgets/reset.graphql")
var UpdateFlexibleBudgetMutation = queries.Get("budgets/flexible_set.graphql")
var UpdateFlexRolloverSettingsMutation = queries.Get("budgets/flex_rollover_set.graphql")

type Budget struct {
	CategoryID   string  `json:"category_id"`
	CategoryName string  `json:"category_name"`
	Planned      float64 `json:"planned"`
	Actual       float64 `json:"actual"`
}

type ListBudgetsOptions struct {
	StartDate string
	EndDate   string
}

func (s *Service) GetBudget(ctx context.Context, categoryID, startDate, endDate string) (*Budget, error) {
	var resp struct {
		BudgetData struct {
			MonthlyAmountsByCategory []struct {
				Category struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"category"`
				MonthlyAmounts []struct {
					Month                 string  `json:"month"`
					PlannedCashFlowAmount float64 `json:"plannedCashFlowAmount"`
					ActualAmount          float64 `json:"actualAmount"`
				} `json:"monthlyAmounts"`
			} `json:"monthlyAmountsByCategory"`
		} `json:"budgetData"`
	}

	variables := map[string]any{
		"startDate": startDate,
		"endDate":   endDate,
	}

	err := s.Client.Do(ctx, &graphql.Request{
		OperationName: "Common_GetJointPlanningData",
		Query:         GetBudgetsQuery,
		Variables:     variables,
	}, &resp)

	if err != nil {
		return nil, err
	}

	for _, cat := range resp.BudgetData.MonthlyAmountsByCategory {
		if cat.Category.ID != categoryID {
			continue
		}
		budget := &Budget{CategoryID: cat.Category.ID, CategoryName: cat.Category.Name}
		if len(cat.MonthlyAmounts) > 0 {
			budget.Planned = cat.MonthlyAmounts[0].PlannedCashFlowAmount
			budget.Actual = cat.MonthlyAmounts[0].ActualAmount
		}
		return budget, nil
	}

	return nil, errors.New(errors.ResourceNotFound, fmt.Sprintf("budget category %s not found", categoryID), errors.CatAPI, false, nil)
}

func (s *Service) UpdateFlexibleBudget(ctx context.Context, month, year int, amount float64) error {
	var resp struct {
		UpdateOrCreateFlexBudgetItem struct {
			FlexBudgetItem struct {
				Month int `json:"month"`
			} `json:"flexBudgetItem"`
		} `json:"updateOrCreateFlexBudgetItem"`
	}

	return s.Client.DoMutation(ctx, &graphql.Request{
		OperationName: "Common_UpdateFlexBudgetMutation",
		Query:         UpdateFlexibleBudgetMutation,
		Variables: map[string]any{
			"input": map[string]any{
				"month":                 month,
				"year":                  year,
				"plannedCashFlowAmount": amount,
			},
		},
	}, &resp)
}

func (s *Service) UpdateFlexRolloverSettings(ctx context.Context, startMonth string, startingBalance float64, enabled bool) error {
	var resp struct {
		UpdateBudgetSettings struct {
			BudgetRolloverPeriod struct {
				ID string `json:"id"`
			} `json:"budgetRolloverPeriod"`
		} `json:"updateBudgetSettings"`
	}

	return s.Client.DoMutation(ctx, &graphql.Request{
		OperationName: "UpdateFlexRolloverSettings",
		Query:         UpdateFlexRolloverSettingsMutation,
		Variables: map[string]any{
			"input": map[string]any{
				"rolloverStartMonth":      startMonth,
				"rolloverStartingBalance": startingBalance,
				"rolloverEnabled":         enabled,
			},
		},
	}, &resp)
}

func (s *Service) ListBudgets(ctx context.Context, opts ListBudgetsOptions) ([]Budget, error) {
	var resp struct {
		BudgetData struct {
			MonthlyAmountsByCategory []struct {
				Category struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"category"`
				MonthlyAmounts []struct {
					Month                 string  `json:"month"`
					PlannedCashFlowAmount float64 `json:"plannedCashFlowAmount"`
					ActualAmount          float64 `json:"actualAmount"`
				} `json:"monthlyAmounts"`
			} `json:"monthlyAmountsByCategory"`
		} `json:"budgetData"`
	}

	variables := map[string]any{
		"startDate": opts.StartDate,
		"endDate":   opts.EndDate,
	}

	err := s.Client.Do(ctx, &graphql.Request{
		OperationName: "Common_GetJointPlanningData",
		Query:         GetBudgetsQuery,
		Variables:     variables,
	}, &resp)

	if err != nil {
		return nil, err
	}

	budgets := make([]Budget, 0, len(resp.BudgetData.MonthlyAmountsByCategory))
	for _, cat := range resp.BudgetData.MonthlyAmountsByCategory {
		for _, m := range cat.MonthlyAmounts {
			budgets = append(budgets, Budget{
				CategoryID:   cat.Category.ID,
				CategoryName: cat.Category.Name,
				Planned:      m.PlannedCashFlowAmount,
				Actual:       m.ActualAmount,
			})
		}
	}

	return budgets, nil
}

func (s *Service) SetBudget(ctx context.Context, categoryID string, amount float64, startDate string) (*Budget, error) {
	var resp struct {
		UpdateOrCreateBudgetItem struct {
			BudgetItem struct {
				ID           string  `json:"id"`
				BudgetAmount float64 `json:"budgetAmount"`
			} `json:"budgetItem"`
		} `json:"updateOrCreateBudgetItem"`
	}

	variables := map[string]any{
		"input": map[string]any{
			"categoryId":    categoryID,
			"amount":        amount,
			"timeframe":     "month",
			"startDate":     startDate,
			"applyToFuture": false,
		},
	}

	err := s.Client.DoMutation(ctx, &graphql.Request{
		OperationName: "Common_UpdateBudgetItem",
		Query:         SetBudgetMutation,
		Variables:     variables,
	}, &resp)

	if err != nil {
		return nil, err
	}

	return &Budget{
		CategoryID: categoryID,
		Planned:    resp.UpdateOrCreateBudgetItem.BudgetItem.BudgetAmount,
	}, nil
}

func (s *Service) ResetBudget(ctx context.Context, month, year int) error {
	var resp struct {
		ResetBudget struct {
			OK bool `json:"ok"`
		} `json:"resetBudget"`
	}

	return s.Client.DoMutation(ctx, &graphql.Request{
		OperationName: "ResetBudget",
		Query:         ResetBudgetMutation,
		Variables:     map[string]any{"month": month, "year": year},
	}, &resp)
}
