package observability

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Metrics struct {
	mu              sync.RWMutex
	requestCount    int64
	requestDuration time.Duration
	totalTokens     int64
	totalCost       float64
	byModel         map[string]int64
	byStrategy      map[string]int64
	errors          int64
	startTime       time.Time
}

func New() *Metrics {
	return &Metrics{
		byModel:    make(map[string]int64),
		byStrategy: make(map[string]int64),
		startTime:  time.Now(),
	}
}

func (m *Metrics) RecordRequest(model, strategy string, tokensIn, tokensOut int, cost float64, duration time.Duration, isError bool) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requestCount++
	m.requestDuration += duration
	m.totalTokens += int64(tokensIn + tokensOut)
	m.totalCost += cost
	m.byModel[model]++
	m.byStrategy[strategy]++
	if isError {
		m.errors++
	}
}

func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.RLock()
		defer m.mu.RUnlock()

		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		avgLatency := float64(0)
		if m.requestCount > 0 {
			avgLatency = float64(m.requestDuration.Milliseconds()) / float64(m.requestCount)
		}

		fmt.Fprintf(w, "# HELP aperture_requests_total Total number of requests\n")
		fmt.Fprintf(w, "# TYPE aperture_requests_total counter\n")
		fmt.Fprintf(w, "aperture_requests_total %d\n", m.requestCount)

		fmt.Fprintf(w, "# HELP aperture_errors_total Total number of errors\n")
		fmt.Fprintf(w, "# TYPE aperture_errors_total counter\n")
		fmt.Fprintf(w, "aperture_errors_total %d\n", m.errors)

		fmt.Fprintf(w, "# HELP aperture_cost_total Total cost in USD\n")
		fmt.Fprintf(w, "# TYPE aperture_cost_total counter\n")
		fmt.Fprintf(w, "aperture_cost_total %.6f\n", m.totalCost)

		fmt.Fprintf(w, "# HELP aperture_tokens_total Total tokens processed\n")
		fmt.Fprintf(w, "# TYPE aperture_tokens_total counter\n")
		fmt.Fprintf(w, "aperture_tokens_total %d\n", m.totalTokens)

		fmt.Fprintf(w, "# HELP aperture_latency_ms_avg Average latency in ms\n")
		fmt.Fprintf(w, "# TYPE aperture_latency_ms_avg gauge\n")
		fmt.Fprintf(w, "aperture_latency_ms_avg %.2f\n", avgLatency)

		fmt.Fprintf(w, "# HELP aperture_uptime_seconds Server uptime\n")
		fmt.Fprintf(w, "# TYPE aperture_uptime_seconds gauge\n")
		fmt.Fprintf(w, "aperture_uptime_seconds %.0f\n", time.Since(m.startTime).Seconds())

		fmt.Fprintf(w, "# HELP aperture_requests_by_model Requests by model\n")
		fmt.Fprintf(w, "# TYPE aperture_requests_by_model counter\n")
		for model, count := range m.byModel {
			fmt.Fprintf(w, `aperture_requests_by_model{model="%s"} %d`+"\n", model, count)
		}

		fmt.Fprintf(w, "# HELP aperture_requests_by_strategy Requests by routing strategy\n")
		fmt.Fprintf(w, "# TYPE aperture_requests_by_strategy counter\n")
		for strategy, count := range m.byStrategy {
			fmt.Fprintf(w, `aperture_requests_by_strategy{strategy="%s"} %d`+"\n", strategy, count)
		}
	})
}
