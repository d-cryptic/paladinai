//go:build ignore

// generate_regression.go generates Stage 10 cost and latency regression fixtures.
// Run with: go run test/fixtures/generate_regression.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type regressionCase struct {
	ID                string `json:"id"`
	Category          string `json:"category"`
	Description       string `json:"description"`
	BaselineTokens    int    `json:"baseline_tokens,omitempty"`
	ObservedTokens    int    `json:"observed_tokens,omitempty"`
	LatencyBudgetMS   int    `json:"latency_budget_ms,omitempty"`
	ObservedLatencyMS int    `json:"observed_latency_ms,omitempty"`
}

func main() {
	if err := writeCost(); err != nil {
		panic(err)
	}
	if err := writeLatency(); err != nil {
		panic(err)
	}
}

func writeCost() error {
	f, err := os.Create("test/fixtures/cost_regression.jsonl")
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	for i := 1; i <= 50; i++ {
		baseline := 900 + i*37
		observed := baseline + (i%9)*10
		if i%17 == 0 {
			observed = baseline + baseline/10
		}
		tc := regressionCase{
			ID:             fmt.Sprintf("cost-reg-%04d", i),
			Category:       "cost_regression",
			Description:    fmt.Sprintf("Stage 10 token budget case %d", i),
			BaselineTokens: baseline,
			ObservedTokens: observed,
		}
		if err := writeJSONL(f, tc); err != nil {
			return err
		}
	}
	return nil
}

func writeLatency() error {
	f, err := os.Create("test/fixtures/latency_budget.jsonl")
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	for i := 1; i <= 50; i++ {
		budget := 3000 + (i%5)*1000
		observed := budget - 250 - (i%7)*35
		if observed < 100 {
			observed = 100
		}
		tc := regressionCase{
			ID:                fmt.Sprintf("latency-budget-%04d", i),
			Category:          "latency_budget",
			Description:       fmt.Sprintf("Stage 10 latency budget case %d", i),
			LatencyBudgetMS:   budget,
			ObservedLatencyMS: observed,
		}
		if err := writeJSONL(f, tc); err != nil {
			return err
		}
	}
	return nil
}

func writeJSONL(f *os.File, tc regressionCase) error {
	b, err := json.Marshal(tc)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}
