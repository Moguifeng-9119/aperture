package store

import (
	"fmt"
	"time"
)

type RoutingDecision struct {
	ID             int64     `json:"id"`
	Timestamp      time.Time `json:"timestamp"`
	RequestID      string    `json:"request_id"`
	ProjectID      string    `json:"project_id"`
	ConversationID string    `json:"conversation_id"`
	Strategy       string    `json:"strategy"`
	Complexity     string    `json:"complexity"`
	Confidence     float64   `json:"confidence"`
	Model          string    `json:"model"`
	Provider       string    `json:"provider"`
	Reason         string    `json:"reason"`
	MessagesJSON   string    `json:"messages_json,omitempty"`
	TokensIn       int       `json:"tokens_in"`
	TokensOut      int       `json:"tokens_out"`
	CostUSD        float64   `json:"cost_usd"`
	SavingUSD      float64   `json:"saving_usd"`
	LatencyMs      int64     `json:"latency_ms"`
	HTTPStatus     int       `json:"http_status"`
	Error          string    `json:"error,omitempty"`
}

func (s *Store) RecordDecision(d *RoutingDecision) error {
	if d.RequestID == "" {
		d.RequestID = fmt.Sprintf("req_%d", time.Now().UnixNano())
	}
	if d.Timestamp.IsZero() {
		d.Timestamp = time.Now()
	}

	_, err := s.db.Exec(`INSERT INTO routing_decisions
		(timestamp, request_id, project_id, conversation_id, strategy, complexity, confidence, model, provider, reason, messages_json, tokens_in, tokens_out, cost_usd, saving_usd, latency_ms, http_status, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.Timestamp, d.RequestID, d.ProjectID, d.ConversationID, d.Strategy,
		d.Complexity, d.Confidence, d.Model, d.Provider, d.Reason, d.MessagesJSON,
		d.TokensIn, d.TokensOut, d.CostUSD, d.SavingUSD, d.LatencyMs, d.HTTPStatus, d.Error)
	if err != nil {
		return fmt.Errorf("record decision: %w", err)
	}
	return nil
}

type AnalyticsSummary struct {
	TotalRequests int                `json:"total_requests"`
	TotalCostUSD  float64            `json:"total_cost_usd"`
	TotalSavingUSD float64           `json:"total_saving_usd"`
	SavingPercent float64            `json:"saving_percent"`
	AvgLatencyMs  float64            `json:"avg_latency_ms"`
	ByModel       []ModelBreakdown   `json:"by_model"`
	ByStrategy    []StrategyBreakdown `json:"by_strategy"`
}

type ModelBreakdown struct {
	Model    string  `json:"model"`
	Requests int     `json:"requests"`
	CostUSD  float64 `json:"cost_usd"`
}

type StrategyBreakdown struct {
	Strategy string `json:"strategy"`
	Requests int    `json:"requests"`
}

func (s *Store) GetAnalyticsSummary(from, to time.Time, projectID string) (*AnalyticsSummary, error) {
	query := `SELECT COUNT(*), COALESCE(SUM(cost_usd), 0), COALESCE(SUM(saving_usd), 0), COALESCE(AVG(latency_ms), 0)
		FROM routing_decisions WHERE timestamp BETWEEN ? AND ?`
	args := []any{from, to}

	if projectID != "" {
		query += " AND project_id = ?"
		args = append(args, projectID)
	}

	var summary AnalyticsSummary
	err := s.db.QueryRow(query, args...).Scan(
		&summary.TotalRequests, &summary.TotalCostUSD, &summary.TotalSavingUSD, &summary.AvgLatencyMs)
	if err != nil {
		return nil, err
	}

	if summary.TotalCostUSD+summary.TotalSavingUSD > 0 {
		summary.SavingPercent = summary.TotalSavingUSD / (summary.TotalCostUSD + summary.TotalSavingUSD) * 100
	}

	modelQuery := `SELECT model, COUNT(*) as cnt, COALESCE(SUM(cost_usd), 0)
		FROM routing_decisions WHERE timestamp BETWEEN ? AND ?`
	if projectID != "" {
		modelQuery += " AND project_id = ?"
	}
	modelQuery += " GROUP BY model ORDER BY cnt DESC"
	rows, err := s.db.Query(modelQuery, args...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var mb ModelBreakdown
			if err := rows.Scan(&mb.Model, &mb.Requests, &mb.CostUSD); err == nil {
				summary.ByModel = append(summary.ByModel, mb)
			}
		}
	}

	stratQuery := `SELECT strategy, COUNT(*) FROM routing_decisions WHERE timestamp BETWEEN ? AND ?`
	if projectID != "" {
		stratQuery += " AND project_id = ?"
	}
	stratQuery += " GROUP BY strategy"
	stratRows, err := s.db.Query(stratQuery, args...)
	if err == nil {
		defer stratRows.Close()
		for stratRows.Next() {
			var sb StrategyBreakdown
			if err := stratRows.Scan(&sb.Strategy, &sb.Requests); err == nil {
				summary.ByStrategy = append(summary.ByStrategy, sb)
			}
		}
	}

	return &summary, nil
}

func (s *Store) ListDecisions(from, to time.Time, projectID string, limit, offset int) ([]*RoutingDecision, int, error) {
	baseQuery := `FROM routing_decisions WHERE timestamp BETWEEN ? AND ?`
	args := []any{from, to}

	if projectID != "" {
		baseQuery += " AND project_id = ?"
		args = append(args, projectID)
	}

	var total int
	countQuery := "SELECT COUNT(*) " + baseQuery
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	selectQuery := `SELECT id, timestamp, request_id, project_id, conversation_id, strategy, complexity, confidence, model, provider, reason, tokens_in, tokens_out, cost_usd, saving_usd, latency_ms, http_status, COALESCE(error, '')
		` + baseQuery + ` ORDER BY timestamp DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.Query(selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var decisions []*RoutingDecision
	for rows.Next() {
		var d RoutingDecision
		if err := rows.Scan(&d.ID, &d.Timestamp, &d.RequestID, &d.ProjectID, &d.ConversationID,
			&d.Strategy, &d.Complexity, &d.Confidence, &d.Model, &d.Provider, &d.Reason,
			&d.TokensIn, &d.TokensOut, &d.CostUSD, &d.SavingUSD, &d.LatencyMs, &d.HTTPStatus, &d.Error); err != nil {
			continue
		}
		decisions = append(decisions, &d)
	}

	return decisions, total, nil
}
