package conversation

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type Session struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	UserID    string    `json:"user_id"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	maxMsgs  int
	ttl      time.Duration
}

func NewStore(maxMessages int, ttl time.Duration) *Store {
	s := &Store{
		sessions: make(map[string]*Session),
		maxMsgs:  maxMessages,
		ttl:      ttl,
	}
	go s.reapLoop()
	return s
}

func (s *Store) GetOrCreate(conversationID, projectID, userID string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	if conversationID != "" {
		if sess, ok := s.sessions[conversationID]; ok {
			sess.UpdatedAt = time.Now()
			return sess
		}
	}

	sess := &Session{
		ID:        uuid.New().String(),
		ProjectID: projectID,
		UserID:    userID,
		Messages:  make([]Message, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.sessions[sess.ID] = sess
	return sess
}

func (s *Store) AddMessages(sessionID string, msgs []Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[sessionID]
	if !ok {
		return
	}

	sess.Messages = append(sess.Messages, msgs...)
	if len(sess.Messages) > s.maxMsgs {
		sess.Messages = sess.Messages[len(sess.Messages)-s.maxMsgs:]
	}
	sess.UpdatedAt = time.Now()
}

func (s *Store) GetMessages(sessionID string, lastN int) []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil
	}

	msgs := sess.Messages
	if lastN > 0 && len(msgs) > lastN {
		msgs = msgs[len(msgs)-lastN:]
	}

	result := make([]Message, len(msgs))
	copy(result, msgs)
	return result
}

func (s *Store) reapLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		cutoff := time.Now().Add(-s.ttl)
		for id, sess := range s.sessions {
			if sess.UpdatedAt.Before(cutoff) {
				delete(s.sessions, id)
			}
		}
		s.mu.Unlock()
	}
}
