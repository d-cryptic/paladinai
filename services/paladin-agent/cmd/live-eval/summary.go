package main

import (
	"fmt"
	"math"
	"sort"
	"time"
)

func summarize(model string, timeoutSeconds int, duration time.Duration, results []caseResult) liveSummary {
	summary := liveSummary{
		Model:          model,
		DurationMS:     duration.Milliseconds(),
		ByCategory:     map[string]categoryStat{},
		Results:        results,
		LiveLLMCalls:   true,
		TimeoutSeconds: timeoutSeconds,
	}
	latencies := make([]time.Duration, 0, len(results))
	categoryLatencies := make(map[string][]time.Duration)

	for _, result := range results {
		summary.Total++
		summary.MeanScore += result.Score
		if result.ProviderError {
			summary.ProviderErrors++
		}
		if result.Pass {
			summary.Passed++
		} else {
			summary.Failed++
		}

		latencies = append(latencies, result.duration)
		categoryLatencies[result.Category] = append(categoryLatencies[result.Category], result.duration)

		stat := summary.ByCategory[result.Category]
		stat.Total++
		stat.MeanScore += result.Score
		if result.Pass {
			stat.Passed++
		} else {
			stat.Failed++
		}
		summary.ByCategory[result.Category] = stat
	}

	if summary.Total > 0 {
		summary.PassRate = float64(summary.Passed) / float64(summary.Total)
		summary.MeanScore /= float64(summary.Total)
	}
	modelScored := summary.Total - summary.ProviderErrors
	if modelScored > 0 {
		summary.ModelPassRate = float64(summary.Passed) / float64(modelScored)
	}
	summary.LatencyMinMS, summary.LatencyP50MS, summary.LatencyP95MS, summary.LatencyMaxMS = latencyStats(latencies)

	for category, stat := range summary.ByCategory {
		if stat.Total > 0 {
			stat.PassRate = float64(stat.Passed) / float64(stat.Total)
			stat.MeanScore /= float64(stat.Total)
		}
		_, stat.LatencyP50MS, stat.LatencyP95MS, _ = latencyStats(categoryLatencies[category])
		summary.ByCategory[category] = stat
	}
	return summary
}

func latencyStats(values []time.Duration) (int64, int64, int64, int64) {
	if len(values) == 0 {
		return 0, 0, 0, 0
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[0].Milliseconds(),
		percentile(sorted, 0.50).Milliseconds(),
		percentile(sorted, 0.95).Milliseconds(),
		sorted[len(sorted)-1].Milliseconds()
}

func percentile(values []time.Duration, p float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(values)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(values) {
		idx = len(values) - 1
	}
	return values[idx]
}

func printText(summary liveSummary) {
	fmt.Printf("model=%s total=%d pass_rate=%.2f mean_score=%.2f p50=%dms p95=%dms max=%dms\n",
		summary.Model, summary.Total, summary.PassRate, summary.MeanScore,
		summary.LatencyP50MS, summary.LatencyP95MS, summary.LatencyMaxMS)
	for category, stat := range summary.ByCategory {
		fmt.Printf("%s total=%d pass_rate=%.2f mean_score=%.2f p50=%dms p95=%dms\n",
			category, stat.Total, stat.PassRate, stat.MeanScore, stat.LatencyP50MS, stat.LatencyP95MS)
	}
}
