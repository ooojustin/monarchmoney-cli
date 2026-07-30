package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func TestGoals(t *testing.T) {
	t.Run("list", testGoalsListJSON)
	t.Run("list_api_error", testGoalsListAPIError)
	t.Run("budgets", testGoalsBudgetsJSON)
}

func testGoalsListJSON(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	exitCode := withReadCommandTestDefaults(t, sessionPath, goalsListCmd)
	saveTestSession(t, sessionPath)

	http.DefaultTransport = testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string `json:"operationName"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "Common_SavingsGoals" {
			t.Fatalf("operation = %q, want Common_SavingsGoals", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"savingsGoals":[{"id":"goal-1","name":"Vacation","type":"savings_goal","status":"active","progress":0.75,"currentBalance":7500,"targetAmount":10000,"plannedMonthlyContribution":500,"isSinkingFund":false,"priority":1},{"id":"goal-2","name":"Emergency Fund","type":"savings_goal","status":"active","progress":0.50,"currentBalance":15000,"targetAmount":30000,"plannedMonthlyContribution":1000,"isSinkingFund":false,"priority":2}]}}`), nil
	})

	out := captureStdout(t, func() {
		goalsListCmd.Run(goalsListCmd, nil)
	})

	if *exitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", *exitCode, out)
	}
	if !strings.Contains(out, `"command":"goals.list"`) {
		t.Fatalf("output missing command = %q", out)
	}
	if !strings.Contains(out, `"Vacation"`) {
		t.Fatalf("output missing Vacation = %q", out)
	}
	if !strings.Contains(out, `"Emergency Fund"`) {
		t.Fatalf("output missing Emergency Fund = %q", out)
	}
	if !strings.Contains(out, `"current_balance":7500`) {
		t.Fatalf("output missing current_balance = %q", out)
	}
	if !strings.Contains(out, `"target_amount":10000`) {
		t.Fatalf("output missing target_amount = %q", out)
	}
}

func testGoalsListAPIError(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	exitCode := withReadCommandTestDefaults(t, sessionPath, goalsListCmd)
	saveTestSession(t, sessionPath)

	http.DefaultTransport = testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil
	})

	out := captureStdout(t, func() {
		goalsListCmd.Run(goalsListCmd, nil)
	})

	if *exitCode == 0 {
		t.Fatalf("exitCode = 0, want API failure; output=%q", out)
	}
	if !strings.Contains(out, `"API_ERROR"`) {
		t.Fatalf("output = %q, want API_ERROR", out)
	}
}

func testGoalsBudgetsJSON(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	exitCode := withReadCommandTestDefaults(t, sessionPath, goalsBudgetsCmd)
	saveTestSession(t, sessionPath)

	http.DefaultTransport = testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string `json:"operationName"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "GetSavingsGoals" {
			t.Fatalf("operation = %q, want GetSavingsGoals", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"savingsGoalMonthlyBudgetAmounts":[{"id":"sgb-1","savingsGoal":{"id":"goal-1","name":"Vacation","type":"savings_goal","status":"active"},"monthlyAmounts":[{"month":"2026-05","plannedAmount":500,"actualAmount":450,"remainingAmount":50}]}]}}`), nil
	})

	monthStr = ""
	_ = goalsBudgetsCmd.Flags().Set("month", "2026-05")
	out := captureStdout(t, func() {
		goalsBudgetsCmd.Run(goalsBudgetsCmd, nil)
	})

	if *exitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", *exitCode, out)
	}
	if !strings.Contains(out, `"command":"goals.budgets"`) {
		t.Fatalf("output missing command = %q", out)
	}
	if !strings.Contains(out, `"goal_name":"Vacation"`) {
		t.Fatalf("output missing goal name = %q", out)
	}
	if !strings.Contains(out, `"planned":500`) {
		t.Fatalf("output missing planned amount = %q", out)
	}
}
