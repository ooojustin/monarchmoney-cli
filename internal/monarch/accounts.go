package monarch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/graphql"
	"github.com/thedavidweng/monarchmoney-cli/internal/money"
	"github.com/thedavidweng/monarchmoney-cli/queries"
)

var GetAccountsQuery = queries.Get("accounts/list.graphql")
var GetAccountQuery = queries.Get("accounts/show.graphql")
var GetAccountHoldingsQuery = queries.Get("accounts/holdings.graphql")
var GetAccountHistoryQuery = queries.Get("accounts/history.graphql")
var GetAccountTypesQuery = queries.Get("accounts/types.graphql")
var GetAccountBalancesAtQuery = queries.Get("accounts/balance_at.graphql")
var RefreshAccountsMutation = queries.Get("accounts/refresh.graphql")
var GetAccountsRefreshStatusQuery = queries.Get("accounts/refresh_status.graphql")
var GetAccountRecentBalancesQuery = queries.Get("accounts/recent_balances.graphql")
var GetSnapshotsByAccountTypeQuery = queries.Get("accounts/snapshots_by_type.graphql")
var GetAggregateSnapshotsQuery = queries.Get("accounts/aggregate_snapshots.graphql")

var UpdateAccountMutation = queries.Get("accounts/update.graphql")
var DeleteAccountMutation = queries.Get("accounts/delete.graphql")
var CreateManualAccountMutation = queries.Get("accounts/create_manual.graphql")
var ParseBalanceHistoryMutation = queries.Get("accounts/parse_balance_history.graphql")
var GetBalanceHistorySessionQuery = queries.Get("accounts/balance_history_session.graphql")

const (
	dateLayout                       = "2006-01-02"
	defaultBalanceHistoryPollDelay   = 10 * time.Second
	defaultBalanceHistoryPollTimeout = 300 * time.Second
)

func createBalanceHistoryFormFile(w *multipart.Writer, field, filename string) (io.Writer, error) {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="`+escapeFormQuotes(field)+`"; filename="`+escapeFormQuotes(filename)+`"`)
	header.Set("Content-Type", "text/csv")
	return w.CreatePart(header)
}

type Account struct {
	ID                              string  `json:"id"`
	DisplayName                     string  `json:"display_name"`
	AccountType                     string  `json:"account_type"`
	TypeGroup                       string  `json:"account_type_group"`
	AccountSubtype                  string  `json:"account_subtype"`
	DisplayBalance                  float64 `json:"display_balance"`
	CurrentBalance                  float64 `json:"current_balance"`
	Limit                           float64 `json:"limit"`
	DataProviderCreditLimit         float64 `json:"data_provider_credit_limit"`
	UpdatedAt                       string  `json:"updated_at"`
	DisplayLastUpdatedAt            string  `json:"display_last_updated_at"`
	DeactivatedAt                   string  `json:"deactivated_at"`
	IsHidden                        bool    `json:"is_hidden"`
	IsAsset                         bool    `json:"is_asset"`
	Mask                            string  `json:"mask"`
	CreatedAt                       string  `json:"created_at"`
	IncludeInNetWorth               bool    `json:"include_in_net_worth"`
	HideFromList                    bool    `json:"hide_from_list"`
	HideTransactionsFromReports     bool    `json:"hide_transactions_from_reports"`
	IncludeBalanceInNetWorth        bool    `json:"include_balance_in_net_worth"`
	IncludeInGoalBalance            bool    `json:"include_in_goal_balance"`
	DataProvider                    string  `json:"data_provider"`
	DataProviderAccountID           string  `json:"data_provider_account_id"`
	IsManual                        bool    `json:"is_manual"`
	TransactionsCount               int     `json:"transactions_count"`
	ManualInvestmentsTrackingMethod string  `json:"manual_investments_tracking_method"`
	Order                           int     `json:"order"`
	Icon                            string  `json:"icon"`
	LogoURL                         string  `json:"logo_url"`
	IsClosed                        bool    `json:"is_closed"`
}

type AccountRecentBalance struct {
	ID               string    `json:"id"`
	DisplayName      string    `json:"display_name"`
	AccountTypeGroup string    `json:"account_type_group"`
	RecentBalances   []float64 `json:"recent_balances"`
}

type AccountBalanceAt struct {
	ID               string  `json:"id"`
	DisplayName      string  `json:"display_name"`
	DisplayBalance   float64 `json:"display_balance"`
	AccountType      string  `json:"account_type"`
	AccountTypeGroup string  `json:"account_type_group"`
}

type Holding struct {
	ID         string  `json:"id"`
	Quantity   float64 `json:"quantity"`
	Basis      float64 `json:"basis"`
	TotalValue float64 `json:"total_value"`
}

type HistoryRecord struct {
	Date   string  `json:"date"`
	Amount float64 `json:"amount"`
}

func (s *Service) GetAccountHoldings(ctx context.Context, accountID string) ([]Holding, error) {
	var resp struct {
		Portfolio struct {
			AggregateHoldings struct {
				Edges []struct {
					Node struct {
						ID         string  `json:"id"`
						Quantity   float64 `json:"quantity"`
						Basis      float64 `json:"basis"`
						TotalValue float64 `json:"totalValue"`
						Holdings   []struct {
							ID       string  `json:"id"`
							Quantity float64 `json:"quantity"`
							Name     string  `json:"name"`
							Ticker   string  `json:"ticker"`
							Account  struct {
								ID string `json:"id"`
							} `json:"account"`
						} `json:"holdings"`
					} `json:"node"`
				} `json:"edges"`
			} `json:"aggregateHoldings"`
		} `json:"portfolio"`
	}

	err := s.Client.Do(ctx, &graphql.Request{
		OperationName: "Web_GetHoldings",
		Query:         GetAccountHoldingsQuery,
	}, &resp)

	if err != nil {
		return nil, err
	}

	holdings := []Holding{}
	for _, edge := range resp.Portfolio.AggregateHoldings.Edges {
		if accountID != "" {
			matched := false
			for _, h := range edge.Node.Holdings {
				if h.Account.ID == accountID {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		node := edge.Node
		holdings = append(holdings, Holding{
			ID:         node.ID,
			Quantity:   node.Quantity,
			Basis:      money.Round2(node.Basis),
			TotalValue: money.Round2(node.TotalValue),
		})
	}

	return holdings, nil
}

type SecurityHolding struct {
	ID        string  `json:"id"`
	Ticker    string  `json:"ticker"`
	Name      string  `json:"name"`
	Quantity  float64 `json:"quantity"`
	Basis     float64 `json:"basis"`
	Value     float64 `json:"value"`
	AccountID string  `json:"account_id"`
}

func (s *Service) ListHoldings(ctx context.Context) ([]SecurityHolding, error) {
	var resp struct {
		Portfolio struct {
			AggregateHoldings struct {
				Edges []struct {
					Node struct {
						Holdings []struct {
							ID        string  `json:"id"`
							Quantity  float64 `json:"quantity"`
							Name      string  `json:"name"`
							Ticker    string  `json:"ticker"`
							Value     float64 `json:"value"`
							CostBasis float64 `json:"costBasis"`
							Account   struct {
								ID string `json:"id"`
							} `json:"account"`
						} `json:"holdings"`
					} `json:"node"`
				} `json:"edges"`
			} `json:"aggregateHoldings"`
		} `json:"portfolio"`
	}

	err := s.Client.Do(ctx, &graphql.Request{
		OperationName: "Web_GetHoldings",
		Query:         GetAccountHoldingsQuery,
	}, &resp)
	if err != nil {
		return nil, err
	}

	var holdings []SecurityHolding
	for _, edge := range resp.Portfolio.AggregateHoldings.Edges {
		for _, h := range edge.Node.Holdings {
			holdings = append(holdings, SecurityHolding{
				ID:        h.ID,
				Ticker:    h.Ticker,
				Name:      h.Name,
				Quantity:  h.Quantity,
				Basis:     money.Round2(h.CostBasis),
				Value:     money.Round2(h.Value),
				AccountID: h.Account.ID,
			})
		}
	}
	return holdings, nil
}

func (s *Service) GetAccountHistory(ctx context.Context, accountID, startDate, endDate string) ([]HistoryRecord, error) {
	var resp struct {
		Account struct {
			ID             string    `json:"id"`
			RecentBalances []float64 `json:"recentBalances"`
		} `json:"account"`
	}

	if startDate == "" || endDate == "" {
		return nil, errors.New(errors.InvalidArguments, "account history requires start and end dates", errors.CatValidation, false, nil)
	}
	start, parseErr := time.ParseInLocation(dateLayout, startDate, time.Local)
	if parseErr != nil {
		return nil, errors.New(errors.InvalidArguments, "--from must use YYYY-MM-DD", errors.CatValidation, false, parseErr)
	}
	end, parseErr := time.ParseInLocation(dateLayout, endDate, time.Local)
	if parseErr != nil {
		return nil, errors.New(errors.InvalidArguments, "--to must use YYYY-MM-DD", errors.CatValidation, false, parseErr)
	}
	if start.After(end) {
		return nil, errors.New(errors.InvalidArguments, "start date must not be after end date", errors.CatValidation, false, nil)
	}

	err := s.Client.Do(ctx, &graphql.Request{
		OperationName: "GetAccountHistory",
		Query:         GetAccountHistoryQuery,
		Variables:     map[string]any{"id": accountID, "startDate": startDate},
	}, &resp)

	if err != nil {
		return nil, err
	}

	if resp.Account.ID == "" {
		return nil, errors.New(errors.ResourceNotFound, fmt.Sprintf("account %s not found", accountID), errors.CatAPI, false, nil)
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	firstDate := today.AddDate(0, 0, -len(resp.Account.RecentBalances)+1)
	history := []HistoryRecord{}
	for i, balance := range resp.Account.RecentBalances {
		date := firstDate.AddDate(0, 0, i)
		if date.Before(start) || date.After(end) {
			continue
		}
		history = append(history, HistoryRecord{Date: date.Format(dateLayout), Amount: money.Round2(balance)})
	}

	return history, nil
}

func (s *Service) GetAccount(ctx context.Context, id string) (*Account, error) {
	var resp struct {
		Account struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
			AccountType struct {
				Name    string `json:"name"`
				Display string `json:"display"`
			} `json:"type"`
			Subtype struct {
				Name    string `json:"name"`
				Display string `json:"display"`
			} `json:"subtype"`
			DisplayBalance                  float64 `json:"displayBalance"`
			CurrentBalance                  float64 `json:"currentBalance"`
			Limit                           float64 `json:"limit"`
			DataProviderCreditLimit         float64 `json:"dataProviderCreditLimit"`
			UpdatedAt                       string  `json:"updatedAt"`
			DisplayLastUpdatedAt            string  `json:"displayLastUpdatedAt"`
			DeactivatedAt                   string  `json:"deactivatedAt"`
			IsHidden                        bool    `json:"isHidden"`
			IsAsset                         bool    `json:"isAsset"`
			Mask                            string  `json:"mask"`
			CreatedAt                       string  `json:"createdAt"`
			IncludeInNetWorth               bool    `json:"includeInNetWorth"`
			HideFromList                    bool    `json:"hideFromList"`
			HideTransactionsFromReports     bool    `json:"hideTransactionsFromReports"`
			IncludeBalanceInNetWorth        bool    `json:"includeBalanceInNetWorth"`
			IncludeInGoalBalance            bool    `json:"includeInGoalBalance"`
			DataProvider                    string  `json:"dataProvider"`
			DataProviderAccountID           string  `json:"dataProviderAccountId"`
			IsManual                        bool    `json:"isManual"`
			TransactionsCount               int     `json:"transactionsCount"`
			ManualInvestmentsTrackingMethod string  `json:"manualInvestmentsTrackingMethod"`
			Order                           int     `json:"order"`
			Icon                            string  `json:"icon"`
			LogoURL                         string  `json:"logoUrl"`
		} `json:"account"`
	}

	err := s.Client.Do(ctx, &graphql.Request{
		OperationName: "AccountDetails_getAccount",
		Query:         GetAccountQuery,
		Variables:     map[string]any{"id": id},
	}, &resp)

	if err != nil {
		return nil, err
	}

	if resp.Account.ID == "" {
		return nil, errors.New(errors.ResourceNotFound, fmt.Sprintf("account %s not found", id), errors.CatAPI, false, nil)
	}

	return &Account{
		ID:                              resp.Account.ID,
		DisplayName:                     resp.Account.DisplayName,
		AccountType:                     resp.Account.AccountType.Name,
		AccountSubtype:                  resp.Account.Subtype.Name,
		DisplayBalance:                  money.Round2(resp.Account.DisplayBalance),
		CurrentBalance:                  money.Round2(resp.Account.CurrentBalance),
		Limit:                           money.Round2(resp.Account.Limit),
		DataProviderCreditLimit:         money.Round2(resp.Account.DataProviderCreditLimit),
		UpdatedAt:                       resp.Account.UpdatedAt,
		DisplayLastUpdatedAt:            resp.Account.DisplayLastUpdatedAt,
		DeactivatedAt:                   resp.Account.DeactivatedAt,
		IsHidden:                        resp.Account.IsHidden,
		IsAsset:                         resp.Account.IsAsset,
		Mask:                            resp.Account.Mask,
		CreatedAt:                       resp.Account.CreatedAt,
		IncludeInNetWorth:               resp.Account.IncludeInNetWorth,
		HideFromList:                    resp.Account.HideFromList,
		HideTransactionsFromReports:     resp.Account.HideTransactionsFromReports,
		IncludeBalanceInNetWorth:        resp.Account.IncludeBalanceInNetWorth,
		IncludeInGoalBalance:            resp.Account.IncludeInGoalBalance,
		DataProvider:                    resp.Account.DataProvider,
		DataProviderAccountID:           resp.Account.DataProviderAccountID,
		IsManual:                        resp.Account.IsManual,
		TransactionsCount:               resp.Account.TransactionsCount,
		ManualInvestmentsTrackingMethod: resp.Account.ManualInvestmentsTrackingMethod,
		Order:                           resp.Account.Order,
		Icon:                            resp.Account.Icon,
		LogoURL:                         resp.Account.LogoURL,
		IsClosed:                        resp.Account.DeactivatedAt != "",
	}, nil
}

func (s *Service) GetAccountRecentBalances(ctx context.Context, startDate string) ([]AccountRecentBalance, error) {
	var resp struct {
		Accounts []struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
			Type        struct {
				Group string `json:"group"`
			} `json:"type"`
			RecentBalances []float64 `json:"recentBalances"`
		} `json:"accounts"`
	}

	err := s.Client.Do(ctx, &graphql.Request{
		OperationName: "GetAccountRecentBalances",
		Query:         GetAccountRecentBalancesQuery,
		Variables:     map[string]any{"startDate": startDate},
	}, &resp)

	if err != nil {
		return nil, err
	}

	out := make([]AccountRecentBalance, len(resp.Accounts))
	for i, a := range resp.Accounts {
		balances := make([]float64, len(a.RecentBalances))
		for j, balance := range a.RecentBalances {
			balances[j] = money.Round2(balance)
		}
		out[i] = AccountRecentBalance{
			ID:               a.ID,
			DisplayName:      a.DisplayName,
			AccountTypeGroup: a.Type.Group,
			RecentBalances:   balances,
		}
	}

	return out, nil
}

func (s *Service) GetAccountBalancesAt(ctx context.Context, date string, accountIDs []string) ([]AccountBalanceAt, error) {
	var resp struct {
		Accounts []struct {
			ID             string  `json:"id"`
			DisplayName    string  `json:"displayName"`
			DisplayBalance float64 `json:"displayBalance"`
			Type           struct {
				Name  string `json:"name"`
				Group string `json:"group"`
			} `json:"type"`
		} `json:"accounts"`
	}

	err := s.Client.Do(ctx, &graphql.Request{
		OperationName: "Common_GetDisplayBalanceAtDate",
		Query:         GetAccountBalancesAtQuery,
		Variables:     map[string]any{"date": date},
	}, &resp)

	if err != nil {
		return nil, err
	}

	filter := map[string]bool{}
	for _, id := range accountIDs {
		filter[id] = true
	}

	out := make([]AccountBalanceAt, 0, len(resp.Accounts))
	for _, a := range resp.Accounts {
		if len(filter) > 0 && !filter[a.ID] {
			continue
		}
		out = append(out, AccountBalanceAt{
			ID:               a.ID,
			DisplayName:      a.DisplayName,
			DisplayBalance:   money.Round2(a.DisplayBalance),
			AccountType:      a.Type.Name,
			AccountTypeGroup: a.Type.Group,
		})
	}

	return out, nil
}

func (s *Service) GetSnapshotsByAccountType(ctx context.Context, startDate, timeframe string) (any, error) {
	var resp struct {
		SnapshotsByAccountType []struct {
			AccountType string  `json:"accountType"`
			Month       string  `json:"month"`
			Balance     float64 `json:"balance"`
		} `json:"snapshotsByAccountType"`
		AccountTypes []struct {
			Name  string `json:"name"`
			Group string `json:"group"`
		} `json:"accountTypes"`
	}

	err := s.Client.Do(ctx, &graphql.Request{
		OperationName: "GetSnapshotsByAccountType",
		Query:         GetSnapshotsByAccountTypeQuery,
		Variables: map[string]any{
			"startDate": startDate,
			"timeframe": timeframe,
		},
	}, &resp)

	if err != nil {
		return nil, err
	}
	for i := range resp.SnapshotsByAccountType {
		resp.SnapshotsByAccountType[i].Balance = money.Round2(resp.SnapshotsByAccountType[i].Balance)
	}

	return resp, nil
}

func (s *Service) GetAggregateSnapshots(ctx context.Context, startDate, endDate, accountType string) (any, error) {
	var resp struct {
		AggregateSnapshots []struct {
			Date    string  `json:"date"`
			Balance float64 `json:"balance"`
		} `json:"aggregateSnapshots"`
	}

	filters := map[string]any{}
	if startDate != "" {
		filters["startDate"] = startDate
	}
	if endDate != "" {
		filters["endDate"] = endDate
	}
	if accountType != "" {
		filters["accountType"] = accountType
	}

	err := s.Client.Do(ctx, &graphql.Request{
		OperationName: "GetAggregateSnapshots",
		Query:         GetAggregateSnapshotsQuery,
		Variables:     map[string]any{"filters": filters},
	}, &resp)

	if err != nil {
		return nil, err
	}
	for i := range resp.AggregateSnapshots {
		resp.AggregateSnapshots[i].Balance = money.Round2(resp.AggregateSnapshots[i].Balance)
	}

	return resp.AggregateSnapshots, nil
}

func (s *Service) GetAccountTypes(ctx context.Context) ([]string, error) {
	var resp struct {
		AccountTypeOptions []struct {
			Name    string `json:"name"`
			Display string `json:"display"`
		} `json:"accountTypes"`
	}

	err := s.Client.Do(ctx, &graphql.Request{
		OperationName: "GetAccountTypeOptions",
		Query:         GetAccountTypesQuery,
	}, &resp)

	if err != nil {
		return nil, err
	}

	types := make([]string, len(resp.AccountTypeOptions))
	for i, t := range resp.AccountTypeOptions {
		types[i] = t.Name
	}

	return types, nil
}

func (s *Service) GetAccountsRefreshStatus(ctx context.Context) (map[string]any, error) {
	var resp struct {
		Accounts []struct {
			ID                string `json:"id"`
			HasSyncInProgress bool   `json:"hasSyncInProgress"`
		} `json:"accounts"`
	}

	err := s.Client.Do(ctx, &graphql.Request{
		OperationName: "ForceRefreshAccountsQuery",
		Query:         GetAccountsRefreshStatusQuery,
	}, &resp)

	if err != nil {
		return nil, err
	}

	accounts := make([]map[string]any, 0, len(resp.Accounts))
	isComplete := true
	for _, account := range resp.Accounts {
		if account.HasSyncInProgress {
			isComplete = false
		}
		accounts = append(accounts, map[string]any{
			"id":                   account.ID,
			"has_sync_in_progress": account.HasSyncInProgress,
		})
	}

	return map[string]any{
		"is_complete": isComplete,
		"status": func() string {
			if isComplete {
				return "complete"
			}
			return "syncing"
		}(),
		"accounts": accounts,
	}, nil
}

func (s *Service) ListAccounts(ctx context.Context) ([]Account, error) {
	var resp struct {
		Accounts []struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
			AccountType struct {
				Name    string `json:"name"`
				Display string `json:"display"`
				Group   string `json:"group"`
			} `json:"type"`
			Subtype struct {
				Name    string `json:"name"`
				Display string `json:"display"`
			} `json:"subtype"`
			DisplayBalance                  float64 `json:"displayBalance"`
			CurrentBalance                  float64 `json:"currentBalance"`
			Limit                           float64 `json:"limit"`
			DataProviderCreditLimit         float64 `json:"dataProviderCreditLimit"`
			UpdatedAt                       string  `json:"updatedAt"`
			DisplayLastUpdatedAt            string  `json:"displayLastUpdatedAt"`
			DeactivatedAt                   string  `json:"deactivatedAt"`
			IsHidden                        bool    `json:"isHidden"`
			IsAsset                         bool    `json:"isAsset"`
			Mask                            string  `json:"mask"`
			CreatedAt                       string  `json:"createdAt"`
			IncludeInNetWorth               bool    `json:"includeInNetWorth"`
			HideFromList                    bool    `json:"hideFromList"`
			HideTransactionsFromReports     bool    `json:"hideTransactionsFromReports"`
			IncludeBalanceInNetWorth        bool    `json:"includeBalanceInNetWorth"`
			IncludeInGoalBalance            bool    `json:"includeInGoalBalance"`
			DataProvider                    string  `json:"dataProvider"`
			DataProviderAccountID           string  `json:"dataProviderAccountId"`
			IsManual                        bool    `json:"isManual"`
			TransactionsCount               int     `json:"transactionsCount"`
			ManualInvestmentsTrackingMethod string  `json:"manualInvestmentsTrackingMethod"`
			Order                           int     `json:"order"`
			Icon                            string  `json:"icon"`
			LogoURL                         string  `json:"logoUrl"`
		} `json:"accounts"`
	}

	err := s.Client.Do(ctx, &graphql.Request{
		OperationName: "GetAccounts",
		Query:         GetAccountsQuery,
	}, &resp)

	if err != nil {
		return nil, err
	}

	accounts := make([]Account, len(resp.Accounts))
	for i := range resp.Accounts {
		a := &resp.Accounts[i]
		accounts[i] = Account{
			ID:                              a.ID,
			DisplayName:                     a.DisplayName,
			AccountType:                     a.AccountType.Name,
			TypeGroup:                       a.AccountType.Group,
			AccountSubtype:                  a.Subtype.Name,
			DisplayBalance:                  money.Round2(a.DisplayBalance),
			CurrentBalance:                  money.Round2(a.CurrentBalance),
			Limit:                           money.Round2(a.Limit),
			DataProviderCreditLimit:         money.Round2(a.DataProviderCreditLimit),
			UpdatedAt:                       a.UpdatedAt,
			DisplayLastUpdatedAt:            a.DisplayLastUpdatedAt,
			DeactivatedAt:                   a.DeactivatedAt,
			IsHidden:                        a.IsHidden,
			IsAsset:                         a.IsAsset,
			Mask:                            a.Mask,
			CreatedAt:                       a.CreatedAt,
			IncludeInNetWorth:               a.IncludeInNetWorth,
			HideFromList:                    a.HideFromList,
			HideTransactionsFromReports:     a.HideTransactionsFromReports,
			IncludeBalanceInNetWorth:        a.IncludeBalanceInNetWorth,
			IncludeInGoalBalance:            a.IncludeInGoalBalance,
			DataProvider:                    a.DataProvider,
			DataProviderAccountID:           a.DataProviderAccountID,
			IsManual:                        a.IsManual,
			IsClosed:                        a.DeactivatedAt != "",
			TransactionsCount:               a.TransactionsCount,
			ManualInvestmentsTrackingMethod: a.ManualInvestmentsTrackingMethod,
			Order:                           a.Order,
			Icon:                            a.Icon,
			LogoURL:                         a.LogoURL,
		}
	}

	return accounts, nil
}

func (s *Service) CreateManualAccount(ctx context.Context, name, accType string, balance float64) (*Account, error) {
	var resp struct {
		CreateManualAccount struct {
			Account struct {
				ID             string  `json:"id"`
				DisplayName    string  `json:"displayName"`
				DisplayBalance float64 `json:"displayBalance"`
			} `json:"account"`
		} `json:"createManualAccount"`
	}

	err := s.Client.DoMutation(ctx, &graphql.Request{
		OperationName: "Web_CreateManualAccount",
		Query:         CreateManualAccountMutation,
		Variables: map[string]any{
			"name":    name,
			"type":    accType,
			"balance": balance,
		},
	}, &resp)

	if err != nil {
		return nil, err
	}

	return &Account{
		ID:             resp.CreateManualAccount.Account.ID,
		DisplayName:    resp.CreateManualAccount.Account.DisplayName,
		DisplayBalance: money.Round2(resp.CreateManualAccount.Account.DisplayBalance),
	}, nil
}

func (s *Service) RefreshAccounts(ctx context.Context, accountIDs []string) error {
	var resp struct {
		RequestAccountsRefresh struct {
			OK bool `json:"ok"`
		} `json:"requestAccountsRefresh"`
	}

	variables := make(map[string]any)
	if len(accountIDs) > 0 {
		variables["accountIds"] = accountIDs
	}

	return s.Client.DoMutation(ctx, &graphql.Request{
		OperationName: "Common_ForceRefreshAccountsMutation",
		Query:         RefreshAccountsMutation,
		Variables:     variables,
	}, &resp)
}

func (s *Service) UpdateAccount(ctx context.Context, id string, name *string, balance *float64) (*Account, error) {
	var resp struct {
		UpdateAccount struct {
			Account struct {
				ID             string  `json:"id"`
				DisplayName    string  `json:"displayName"`
				DisplayBalance float64 `json:"displayBalance"`
			} `json:"account"`
		} `json:"updateAccount"`
	}

	variables := map[string]any{"id": id}
	if name != nil {
		variables["displayName"] = *name
	}
	if balance != nil {
		variables["balance"] = *balance
	}

	err := s.Client.DoMutation(ctx, &graphql.Request{
		OperationName: "Common_UpdateAccount",
		Query:         UpdateAccountMutation,
		Variables:     variables,
	}, &resp)

	if err != nil {
		return nil, err
	}

	return &Account{
		ID:             resp.UpdateAccount.Account.ID,
		DisplayName:    resp.UpdateAccount.Account.DisplayName,
		DisplayBalance: money.Round2(resp.UpdateAccount.Account.DisplayBalance),
	}, nil
}

func (s *Service) DeleteAccount(ctx context.Context, id string) error {
	var resp struct {
		DeleteAccount struct {
			OK bool `json:"ok"`
		} `json:"deleteAccount"`
	}

	return s.Client.DoMutation(ctx, &graphql.Request{
		OperationName: "Common_DeleteAccount",
		Query:         DeleteAccountMutation,
		Variables:     map[string]any{"id": id},
	}, &resp)
}

func (s *Service) UploadAccountBalanceHistory(ctx context.Context, id string, r io.Reader) error {
	csv, err := io.ReadAll(r)
	if err != nil {
		return errors.New(errors.InternalError, "failed to read balance history CSV", errors.CatInternal, false, err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := createBalanceHistoryFormFile(writer, "files", "upload.csv")
	if err != nil {
		return err
	}
	if _, err := part.Write(csv); err != nil {
		return errors.New(errors.InternalError, "failed to write balance history CSV", errors.CatInternal, false, err)
	}
	mapping, err := json.Marshal(map[string]string{"upload.csv": id})
	if err != nil {
		return errors.New(errors.InternalError, "failed to encode account mapping", errors.CatInternal, false, err)
	}
	if err := writer.WriteField("account_files_mapping", string(mapping)); err != nil {
		return errors.New(errors.InternalError, "failed to write account mapping", errors.CatInternal, false, err)
	}
	if err := writer.Close(); err != nil {
		return errors.New(errors.InternalError, "failed to finalize upload body", errors.CatInternal, false, err)
	}
	sessionKey, err := s.postBalanceHistoryCSV(ctx, body, writer.FormDataContentType())
	if err != nil {
		return err
	}

	var parseResp struct {
		ParseBalanceHistory struct {
			UploadBalanceHistorySession struct {
				Status string `json:"status"`
			} `json:"uploadBalanceHistorySession"`
		} `json:"parseBalanceHistory"`
	}
	err = s.Client.DoMutation(ctx, &graphql.Request{
		OperationName: "Web_ParseUploadBalanceHistorySession",
		Query:         ParseBalanceHistoryMutation,
		Variables:     map[string]any{"input": map[string]any{"sessionKey": sessionKey}},
	}, &parseResp)
	if err != nil {
		return err
	}
	if parseResp.ParseBalanceHistory.UploadBalanceHistorySession.Status == "completed" {
		return nil
	}

	delay := s.balanceHistoryPollDelay
	if delay <= 0 {
		delay = defaultBalanceHistoryPollDelay
	}
	timeout := s.balanceHistoryPollTimeout
	if timeout <= 0 {
		timeout = defaultBalanceHistoryPollTimeout
	}
	ticker := time.NewTicker(delay)
	defer ticker.Stop()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for {
		select {
		case <-ctx.Done():
			return errors.New(errors.NetworkTimeout, "request canceled while waiting for balance history parse", errors.CatNetwork, false, ctx.Err())
		case <-deadline.C:
			return errors.New(errors.APIError, "balance history upload session did not complete in time", errors.CatAPI, true, nil)
		case <-ticker.C:
		}

		var statusResp struct {
			UploadBalanceHistorySession struct {
				Status string `json:"status"`
			} `json:"uploadBalanceHistorySession"`
		}
		err := s.Client.Do(ctx, &graphql.Request{
			OperationName: "Web_GetUploadBalanceHistorySession",
			Query:         GetBalanceHistorySessionQuery,
			Variables:     map[string]any{"sessionKey": sessionKey},
		}, &statusResp)
		if err != nil {
			return err
		}
		if statusResp.UploadBalanceHistorySession.Status == "completed" {
			return nil
		}
	}
}

func (s *Service) postBalanceHistoryCSV(ctx context.Context, body *bytes.Buffer, contentType string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", s.balanceHistoryUploadEndpoint(), body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Client-Platform", "web")
	req.Header.Set("User-Agent", graphql.UserAgent())
	if token := s.Client.TokenValue(); token != "" {
		req.Header.Set("Authorization", "Token "+token)
	}
	if deviceUUID := s.Client.DeviceUUIDValue(); deviceUUID != "" {
		req.Header.Set("device-uuid", deviceUUID)
	}

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return "", errors.New(errors.NetworkUnreachable, "failed to reach balance history upload endpoint", errors.CatNetwork, true, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", errors.New(errors.APIError, fmt.Sprintf("upload failed with status %d", resp.StatusCode), errors.CatAPI, false, nil)
	}

	var payload struct {
		SessionKey string `json:"session_key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil || payload.SessionKey == "" {
		return "", errors.New(errors.APISchemaChanged, "balance history upload response missing session_key", errors.CatAPI, false, err)
	}
	return payload.SessionKey, nil
}
