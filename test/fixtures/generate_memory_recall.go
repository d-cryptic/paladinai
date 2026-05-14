//go:build ignore

// generate_memory_recall.go generates Stage 10 memory recall fixtures.
// Run with: go run test/fixtures/generate_memory_recall.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type memoryRecallCase struct {
	ID                  string   `json:"id"`
	Category            string   `json:"category"`
	Description         string   `json:"description"`
	ExpectedIncidentIDs []string `json:"expected_incident_ids"`
	RecalledIncidentIDs []string `json:"recalled_incident_ids"`
}

func main() {
	f, err := os.Create("test/fixtures/memory_recall.jsonl")
	if err != nil {
		panic(err)
	}
	defer func() { _ = f.Close() }()

	services := []string{"checkout", "payments", "auth", "api", "worker", "database"}
	for i := 1; i <= 150; i++ {
		service := services[(i-1)%len(services)]
		expected := []string{
			fmt.Sprintf("inc-%s-%04d", service, i),
			fmt.Sprintf("inc-%s-%04d", service, i+1000),
			fmt.Sprintf("inc-%s-%04d", service, i+2000),
		}
		recalled := []string{
			expected[0],
			expected[1],
			fmt.Sprintf("inc-neighbor-%04d", i),
			expected[2],
			fmt.Sprintf("inc-noise-%04d", i),
		}
		tc := memoryRecallCase{
			ID:                  fmt.Sprintf("memory-recall-%04d", i),
			Category:            "memory_recall",
			Description:         fmt.Sprintf("Recall prior %s incidents for Stage 10 case %d", service, i),
			ExpectedIncidentIDs: expected,
			RecalledIncidentIDs: recalled,
		}
		b, err := json.Marshal(tc)
		if err != nil {
			panic(err)
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			panic(err)
		}
	}
}
