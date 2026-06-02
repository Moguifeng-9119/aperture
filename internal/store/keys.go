package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

type APIKey struct {
	ID              string    `json:"id"`
	Prefix          string    `json:"prefix"`
	Name            string    `json:"name"`
	ProjectID       string    `json:"project_id"`
	RateLimitRPM    int       `json:"rate_limit_rpm"`
	BudgetUSD       float64   `json:"budget_monthly_usd"`
	AllowedModels   []string  `json:"allowed_models"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	LastUsedAt      *time.Time `json:"last_used_at"`
}

const keyPrefix = "ak_"

func GenerateAPIKey() (id, rawKey string, err error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	rawKey = keyPrefix + hex.EncodeToString(bytes)
	id = "key_" + hex.EncodeToString(bytes[:8])
	return id, rawKey, nil
}

func hashKey(rawKey string) string {
	h := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(h[:])
}

func (s *Store) CreateAPIKey(name, projectID string, rateLimit int, budget float64) (*APIKey, string, error) {
	id, rawKey, err := GenerateAPIKey()
	if err != nil {
		return nil, "", err
	}

	key := &APIKey{
		ID:            id,
		Prefix:        rawKey[:12] + "...",
		Name:          name,
		ProjectID:     projectID,
		RateLimitRPM:  rateLimit,
		BudgetUSD:     budget,
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	_, err = s.db.Exec(`INSERT INTO api_keys (id, key_hash, prefix, name, project_id, rate_limit_rpm, budget_monthly_usd, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, hashKey(rawKey), key.Prefix, name, projectID, rateLimit, budget, true)
	if err != nil {
		return nil, "", fmt.Errorf("insert api key: %w", err)
	}

	return key, rawKey, nil
}

func (s *Store) ValidateAPIKey(rawKey string) (*APIKey, error) {
	hash := hashKey(rawKey)

	var key APIKey
	var lastUsed sql.NullTime
	err := s.db.QueryRow(`SELECT id, prefix, name, project_id, rate_limit_rpm, budget_monthly_usd, is_active, last_used_at
		FROM api_keys WHERE key_hash = ?`, hash).Scan(
		&key.ID, &key.Prefix, &key.Name, &key.ProjectID, &key.RateLimitRPM, &key.BudgetUSD, &key.IsActive, &lastUsed)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if lastUsed.Valid {
		key.LastUsedAt = &lastUsed.Time
	}

	if !key.IsActive {
		return nil, nil
	}

	s.db.Exec(`UPDATE api_keys SET last_used_at = ? WHERE id = ?`, time.Now(), key.ID)
	return &key, nil
}

func (s *Store) ListAPIKeys() ([]*APIKey, error) {
	rows, err := s.db.Query(`SELECT id, prefix, name, project_id, rate_limit_rpm, budget_monthly_usd, is_active, created_at, last_used_at
		FROM api_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*APIKey
	for rows.Next() {
		var k APIKey
		var lastUsed sql.NullTime
		if err := rows.Scan(&k.ID, &k.Prefix, &k.Name, &k.ProjectID, &k.RateLimitRPM, &k.BudgetUSD, &k.IsActive, &k.CreatedAt, &lastUsed); err != nil {
			continue
		}
		if lastUsed.Valid {
			k.LastUsedAt = &lastUsed.Time
		}
		keys = append(keys, &k)
	}
	return keys, nil
}

func (s *Store) DeleteAPIKey(id string) error {
	_, err := s.db.Exec(`DELETE FROM api_keys WHERE id = ?`, id)
	return err
}

type APIKeyStore struct {
	store *Store
}

func NewAPIKeyStore(s *Store) *APIKeyStore {
	return &APIKeyStore{store: s}
}

func (k *APIKeyStore) Validate(rawKey string) (*APIKey, error) {
	return k.store.ValidateAPIKey(rawKey)
}
