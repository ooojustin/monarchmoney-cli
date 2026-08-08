package cli

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

// The inverse of a money list: any numeric leaf not named here must be an exact cent value.
var nonMonetaryJSONKeys = map[string]struct{}{
	"savings_rate":         {},
	"progress":             {},
	"total_change_percent": {},
	"return_percent":       {},
	"quantity":             {},
	"current_price":        {},
	"count":                {},
	"total":                {},
	"transaction_total":    {},
	"account_count":        {},
	"order":                {},
	"transactions_count":   {},
	"holdings_count":       {},
}

func decimalPlaces(number json.Number) int {
	value, err := number.Float64()
	if err != nil {
		return 0
	}
	_, fraction, ok := strings.Cut(strconv.FormatFloat(value, 'f', -1, 64), ".")
	if !ok {
		return 0
	}
	return len(fraction)
}

func assertTwoDecimals(t *testing.T, key string, value any) {
	t.Helper()

	switch v := value.(type) {
	case map[string]any:
		for childKey, child := range v {
			assertTwoDecimals(t, childKey, child)
		}
	case []any:
		for _, child := range v {
			assertTwoDecimals(t, key, child)
		}
	case json.Number:
		if _, skip := nonMonetaryJSONKeys[key]; skip {
			return
		}
		if decimalPlaces(v) > 2 {
			t.Errorf("field %q = %s carries more than 2 decimal places", key, v)
		}
	}
}

func artifactTransport(t *testing.T, payloads map[string]string) http.RoundTripper {
	t.Helper()

	return testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		gqlReq := decodeGraphQLRequest(t, req)
		payload, ok := payloads[gqlReq.OperationName]
		if !ok {
			t.Fatalf("unexpected operation %q", gqlReq.OperationName)
		}
		return testutil.JSONResponse(payload), nil
	})
}

// Balances chosen so the net worth sum is not representable in binary floating point.
const artifactAccountsPayload = `{"data":{"accounts":[
	{"id":"a1","displayName":"Checking","type":{"name":"depository"},"subtype":{"name":"checking"},"displayBalance":107150.61,"currentBalance":107150.61,"isAsset":true,"includeInNetWorth":true,"includeBalanceInNetWorth":true},
	{"id":"a2","displayName":"Savings","type":{"name":"depository"},"subtype":{"name":"savings"},"displayBalance":57.33,"currentBalance":57.33,"isAsset":true,"includeInNetWorth":true,"includeBalanceInNetWorth":true}
]}}`

func TestJSONMoneyFieldsCarryAtMostTwoDecimals(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		payloads map[string]string
	}{
		{
			name: "overview",
			args: []string{"--json", "overview", "--from", "2026-05-01", "--to", "2026-05-31"},
			payloads: map[string]string{
				"GetAccounts":            artifactAccountsPayload,
				"GetCashflowSummary":     `{"data":{"aggregates":[{"summary":{"sumIncome":8500.100000000002,"sumExpense":-1680.1000000000001,"savings":6820.000000000001,"savingsRate":0.2706}}]}}`,
				"GetTransactionsList":    `{"data":{"allTransactions":{"totalCount":1,"results":[{"id":"t1","date":"2026-05-01","amount":-28.560000000000002,"merchant":{"name":"Shop"},"category":{"id":"c1","name":"Food"}}]}}}`,
				"Web_GetTransactionList": `{"data":{"allTransactions":{"totalCount":0,"results":[]}}}`,
			},
		},
		{
			name: "accounts list",
			args: []string{"--json", "accounts", "list"},
			payloads: map[string]string{
				"GetAccounts": `{"data":{"accounts":[{"id":"a1","displayName":"Broker","type":{"name":"brokerage"},"subtype":{"name":"brokerage"},"displayBalance":159.92000000000002,"currentBalance":159.92000000000002,"limit":26500.000000000004,"isAsset":true}]}}`,
			},
		},
		{
			name: "investments portfolio",
			args: []string{"--json", "investments", "portfolio"},
			payloads: map[string]string{
				"Web_GetPortfolio": `{"data":{"portfolio":{"performance":{"totalValue":159.92000000000002,"totalChangePercent":0.1234567,"totalChangeDollars":12.340000000000002},"aggregateHoldings":{"edges":[{"node":{"id":"h1","quantity":12.3456,"basis":100.10000000000001,"totalValue":159.92000000000002,"security":{"id":"s1","name":"Acme","ticker":"ACME","currentPrice":13.955}}}]}}}}`,
			},
		},
		{
			name: "cashflow summary",
			args: []string{"--json", "cashflow", "summary", "--from", "2026-05-01", "--to", "2026-05-31"},
			payloads: map[string]string{
				"GetCashflowSummary": `{"data":{"aggregates":[{"summary":{"sumIncome":8500.100000000002,"sumExpense":-1680.1000000000001,"savings":6820.000000000001,"savingsRate":0.2706}}]}}`,
			},
		},
		{
			name: "cashflow spending",
			args: []string{"--json", "cashflow", "spending", "--from", "2026-05-01", "--to", "2026-05-31"},
			payloads: map[string]string{
				"GetCashflowCategories": `{"data":{"aggregates":[{"groupBy":{"category":{"id":"c1","name":"Food"}},"summary":{"sum":-28.560000000000002}},{"groupBy":{"category":{"id":"c2","name":"Pay"}},"summary":{"sum":107150.61000000002}}]}}`,
			},
		},
		{
			name: "transactions summary",
			args: []string{"--json", "transactions", "summary"},
			payloads: map[string]string{
				"GetTransactionsPage": `{"data":{"aggregates":[{"summary":{"avg":-28.560000000000002,"count":3,"max":107150.61000000002,"maxExpense":-1680.1000000000001,"sum":6820.000000000001,"sumIncome":8500.100000000002,"sumExpense":-1680.1000000000001,"first":"2026-05-01","last":"2026-05-31"}}]}}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newJSONCommandHarness(t, artifactTransport(t, tt.payloads))
			if err := h.execute(tt.args...); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if h.ExitCode != 0 {
				t.Fatalf("exitCode = %d; output=%q", h.ExitCode, h.Stdout.String())
			}

			var env struct {
				Data any `json:"data"`
			}
			decoder := json.NewDecoder(strings.NewReader(h.Stdout.String()))
			decoder.UseNumber()
			if err := decoder.Decode(&env); err != nil {
				t.Fatalf("decode envelope: %v; output=%q", err, h.Stdout.String())
			}
			assertTwoDecimals(t, "data", env.Data)
		})
	}
}
