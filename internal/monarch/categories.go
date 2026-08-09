package monarch

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/graphql"
	"github.com/thedavidweng/monarchmoney-cli/internal/money"
	"github.com/thedavidweng/monarchmoney-cli/queries"
)

var GetCategoriesQuery = queries.Get("categories/list.graphql")
var GetCategoryGroupsQuery = queries.Get("categories/groups.graphql")
var CreateCategoryMutation = queries.Get("categories/create.graphql")
var DeleteCategoryMutation = queries.Get("categories/delete.graphql")
var DeleteCategoriesMutation = queries.Get("categories/delete_many.graphql")
var UpdateCategoryMutation = queries.Get("categories/update.graphql")
var GetCategoryRolloverQuery = queries.Get("categories/rollover.graphql")
var UpdateCategoryGroupMutation = queries.Get("categories/update_group.graphql")

type Category struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	GroupName string `json:"group_name"`
	GroupID   string `json:"group_id"`
	GroupType string `json:"group_type"`
	Order     int    `json:"order"`
	Icon      string `json:"icon"`
}

type CategoryGroup struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	Categories []Category `json:"categories,omitempty"`
}

func (s *Service) ListCategoryGroups(ctx context.Context) ([]CategoryGroup, error) {
	var resp struct {
		CategoryGroups []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Type       string `json:"type"`
			Categories []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"categories"`
		} `json:"categoryGroups"`
	}

	err := s.Client.Do(ctx, &graphql.Request{
		OperationName: "ManageGetCategoryGroups",
		Query:         GetCategoryGroupsQuery,
	}, &resp)

	if err != nil {
		return nil, err
	}

	groups := make([]CategoryGroup, len(resp.CategoryGroups))
	for i, g := range resp.CategoryGroups {
		cats := make([]Category, len(g.Categories))
		for j, c := range g.Categories {
			cats[j] = Category{ID: c.ID, Name: c.Name}
		}
		groups[i] = CategoryGroup{
			ID:         g.ID,
			Name:       g.Name,
			Type:       g.Type,
			Categories: cats,
		}
	}

	return groups, nil
}

func (s *Service) ListCategories(ctx context.Context) ([]Category, error) {
	var resp struct {
		Categories []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Order int    `json:"order"`
			Icon  string `json:"icon"`
			Group struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"group"`
		} `json:"categories"`
	}

	err := s.Client.Do(ctx, &graphql.Request{
		OperationName: "GetCategories",
		Query:         GetCategoriesQuery,
	}, &resp)

	if err != nil {
		return nil, err
	}

	cats := make([]Category, len(resp.Categories))
	for i, c := range resp.Categories {
		cats[i] = Category{
			ID:        c.ID,
			Name:      c.Name,
			GroupName: c.Group.Name,
			GroupID:   c.Group.ID,
			GroupType: c.Group.Type,
			Order:     c.Order,
			Icon:      c.Icon,
		}
	}

	return cats, nil
}

func (s *Service) CreateCategory(ctx context.Context, name, groupID string) (*Category, error) {
	var resp struct {
		CreateCategory struct {
			Category struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"category"`
		} `json:"createCategory"`
	}

	err := s.Client.DoMutation(ctx, &graphql.Request{
		OperationName: "Web_CreateCategory",
		Query:         CreateCategoryMutation,
		Variables: map[string]any{
			"name":    name,
			"groupId": groupID,
		},
	}, &resp)

	if err != nil {
		return nil, err
	}

	return &Category{
		ID:   resp.CreateCategory.Category.ID,
		Name: resp.CreateCategory.Category.Name,
	}, nil
}

func (s *Service) DeleteCategory(ctx context.Context, id string) error {
	var resp struct {
		DeleteCategory struct {
			OK bool `json:"ok"`
		} `json:"deleteCategory"`
	}

	return s.Client.DoMutation(ctx, &graphql.Request{
		OperationName: "Web_DeleteCategory",
		Query:         DeleteCategoryMutation,
		Variables:     map[string]any{"id": id},
	}, &resp)
}

func (s *Service) DeleteCategories(ctx context.Context, ids []string) error {
	var resp struct {
		DeleteTransactionCategories struct {
			OK bool `json:"ok"`
		} `json:"deleteTransactionCategories"`
	}

	return s.Client.DoMutation(ctx, &graphql.Request{
		OperationName: "DeleteCategories",
		Query:         DeleteCategoriesMutation,
		Variables:     map[string]any{"ids": ids},
	}, &resp)
}

type CategoryRollover struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	StartMonth      string  `json:"start_month"`
	StartingBalance float64 `json:"starting_balance"`
	Type            string  `json:"type"`
	Frequency       string  `json:"frequency"`
	TargetAmount    float64 `json:"target_amount"`
}

type UpdateCategoryOptions struct {
	Name              *string
	Icon              *string
	BudgetVariability *string
	ExcludeFromBudget *bool
}

func (s *Service) UpdateCategory(ctx context.Context, categoryID string, opts UpdateCategoryOptions) (*Category, error) {
	input := map[string]any{"id": categoryID}
	if opts.Name != nil {
		input["name"] = *opts.Name
	}
	if opts.Icon != nil {
		input["icon"] = *opts.Icon
	}
	if opts.BudgetVariability != nil {
		input["budgetVariability"] = *opts.BudgetVariability
	}
	if opts.ExcludeFromBudget != nil {
		input["excludeFromBudget"] = *opts.ExcludeFromBudget
	}

	var resp struct {
		UpdateCategory struct {
			Errors []struct {
				Message string `json:"message"`
				Code    string `json:"code"`
			} `json:"errors"`
			Category struct {
				ID                string `json:"id"`
				Name              string `json:"name"`
				Icon              string `json:"icon"`
				BudgetVariability string `json:"budgetVariability"`
				ExcludeFromBudget bool   `json:"excludeFromBudget"`
				Group             struct {
					ID   string `json:"id"`
					Type string `json:"type"`
				} `json:"group"`
			} `json:"category"`
		} `json:"updateCategory"`
	}

	err := s.Client.DoMutation(ctx, &graphql.Request{
		OperationName: "Web_UpdateCategory",
		Query:         UpdateCategoryMutation,
		Variables:     map[string]any{"input": input},
	}, &resp)
	if err != nil {
		return nil, err
	}

	if len(resp.UpdateCategory.Errors) > 0 {
		return nil, fmt.Errorf("update category failed: %s", resp.UpdateCategory.Errors[0].Message)
	}

	return &Category{
		ID:        resp.UpdateCategory.Category.ID,
		Name:      resp.UpdateCategory.Category.Name,
		Icon:      resp.UpdateCategory.Category.Icon,
		GroupID:   resp.UpdateCategory.Category.Group.ID,
		GroupType: resp.UpdateCategory.Category.Group.Type,
	}, nil
}

func (s *Service) GetCategoryRollover(ctx context.Context, categoryID string) (*CategoryRollover, error) {
	var resp struct {
		Category struct {
			ID             string `json:"id"`
			Name           string `json:"name"`
			RolloverPeriod *struct {
				ID              string  `json:"id"`
				StartMonth      string  `json:"startMonth"`
				StartingBalance float64 `json:"startingBalance"`
				Type            string  `json:"type"`
				Frequency       string  `json:"frequency"`
				TargetAmount    float64 `json:"targetAmount"`
			} `json:"rolloverPeriod"`
		} `json:"category"`
	}

	err := s.Client.Do(ctx, &graphql.Request{
		OperationName: "GetCategoryRollover",
		Query:         GetCategoryRolloverQuery,
		Variables:     map[string]any{"id": categoryID},
	}, &resp)
	if err != nil {
		var cliErr *errors.Error
		if stderrors.As(err, &cliErr) && cliErr.Code == errors.APIError && strings.TrimSpace(cliErr.Message) == "Not found" {
			return nil, errors.New(errors.ResourceNotFound, fmt.Sprintf("category %s not found", categoryID), errors.CatAPI, false, err)
		}
		return nil, err
	}

	if resp.Category.ID == "" {
		return nil, errors.New(errors.ResourceNotFound, fmt.Sprintf("category %s not found", categoryID), errors.CatAPI, false, nil)
	}

	r := &CategoryRollover{ID: resp.Category.ID, Name: resp.Category.Name}
	if resp.Category.RolloverPeriod != nil {
		r.StartMonth = resp.Category.RolloverPeriod.StartMonth
		r.StartingBalance = money.Round2(resp.Category.RolloverPeriod.StartingBalance)
		r.Type = resp.Category.RolloverPeriod.Type
		r.Frequency = resp.Category.RolloverPeriod.Frequency
		r.TargetAmount = money.Round2(resp.Category.RolloverPeriod.TargetAmount)
	}
	return r, nil
}

type UpdateCategoryGroupOptions struct {
	Name                       *string
	BudgetVariability          *string
	GroupLevelBudgetingEnabled *bool
	RolloverEnabled            *bool
	RolloverStartMonth         *string
	RolloverStartingBalance    *float64
	RolloverType               *string
}

func (s *Service) UpdateCategoryGroup(ctx context.Context, groupID string, opts UpdateCategoryGroupOptions) (*CategoryGroup, error) {
	input := map[string]any{"id": groupID}
	if opts.Name != nil {
		input["name"] = *opts.Name
	}
	if opts.BudgetVariability != nil {
		input["budgetVariability"] = *opts.BudgetVariability
	}
	if opts.GroupLevelBudgetingEnabled != nil {
		input["groupLevelBudgetingEnabled"] = *opts.GroupLevelBudgetingEnabled
	}
	if opts.RolloverEnabled != nil {
		input["rolloverEnabled"] = *opts.RolloverEnabled
	}
	if opts.RolloverStartMonth != nil {
		input["rolloverStartMonth"] = *opts.RolloverStartMonth
	}
	if opts.RolloverStartingBalance != nil {
		input["rolloverStartingBalance"] = *opts.RolloverStartingBalance
	}
	if opts.RolloverType != nil {
		input["rolloverType"] = *opts.RolloverType
	}

	var resp struct {
		UpdateCategoryGroup struct {
			CategoryGroup struct {
				ID                         string `json:"id"`
				Name                       string `json:"name"`
				Order                      int    `json:"order"`
				Type                       string `json:"type"`
				Color                      string `json:"color"`
				GroupLevelBudgetingEnabled bool   `json:"groupLevelBudgetingEnabled"`
				BudgetVariability          string `json:"budgetVariability"`
			} `json:"categoryGroup"`
		} `json:"updateCategoryGroup"`
	}

	err := s.Client.DoMutation(ctx, &graphql.Request{
		OperationName: "Common_UpdateCategoryGroup",
		Query:         UpdateCategoryGroupMutation,
		Variables:     map[string]any{"input": input},
	}, &resp)
	if err != nil {
		return nil, err
	}

	return &CategoryGroup{
		ID:   resp.UpdateCategoryGroup.CategoryGroup.ID,
		Name: resp.UpdateCategoryGroup.CategoryGroup.Name,
		Type: resp.UpdateCategoryGroup.CategoryGroup.Type,
	}, nil
}
