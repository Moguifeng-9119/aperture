package conversation

import (
	"testing"
	"time"
)

func TestStore_GetOrCreate_New(t *testing.T) {
	s := NewStore(50, 24*time.Hour)

	sess := s.GetOrCreate("", "proj-1", "user-1")
	if sess == nil {
		t.Fatal("expected non-nil session")
	}
	if sess.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if sess.ProjectID != "proj-1" {
		t.Errorf("expected project 'proj-1', got %q", sess.ProjectID)
	}
}

func TestStore_GetOrCreate_Existing(t *testing.T) {
	s := NewStore(50, 24*time.Hour)

	sess1 := s.GetOrCreate("", "proj-1", "user-1")
	sess2 := s.GetOrCreate(sess1.ID, "", "")

	if sess2.ID != sess1.ID {
		t.Errorf("expected same session, got different IDs: %q vs %q", sess1.ID, sess2.ID)
	}
}

func TestStore_AddMessages(t *testing.T) {
	s := NewStore(50, 24*time.Hour)

	sess := s.GetOrCreate("", "proj-1", "user-1")

	msgs := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}
	s.AddMessages(sess.ID, msgs)

	retrieved := s.GetMessages(sess.ID, 100)
	if len(retrieved) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(retrieved))
	}
	if retrieved[0].Role != "user" {
		t.Errorf("expected first message role 'user', got %q", retrieved[0].Role)
	}
	if retrieved[1].Content != "Hi there!" {
		t.Errorf("expected second message content 'Hi there!', got %q", retrieved[1].Content)
	}
}

func TestStore_AddMessages_NonExistentSession(t *testing.T) {
	s := NewStore(50, 24*time.Hour)
	// Should not panic
	s.AddMessages("no-such-session", []Message{{Role: "user", Content: "test"}})
}

func TestStore_GetMessages_NonExistentSession(t *testing.T) {
	s := NewStore(50, 24*time.Hour)
	msgs := s.GetMessages("no-such-session", 10)
	if msgs != nil {
		t.Errorf("expected nil for non-existent session, got %v", msgs)
	}
}

func TestStore_MaxMessagesEviction(t *testing.T) {
	maxMsgs := 5
	s := NewStore(maxMsgs, 24*time.Hour)

	sess := s.GetOrCreate("", "proj-1", "user-1")

	for i := 0; i < 10; i++ {
		s.AddMessages(sess.ID, []Message{
			{Role: "user", Content: "msg"},
		})
	}

	retrieved := s.GetMessages(sess.ID, 100)
	if len(retrieved) > maxMsgs {
		t.Errorf("expected at most %d messages, got %d", maxMsgs, len(retrieved))
	}
}

func TestStore_GetMessages_LastN(t *testing.T) {
	s := NewStore(50, 24*time.Hour)
	sess := s.GetOrCreate("", "proj-1", "user-1")

	for i := 0; i < 10; i++ {
		s.AddMessages(sess.ID, []Message{
			{Role: "user", Content: "msg"},
		})
	}

	retrieved := s.GetMessages(sess.ID, 3)
	if len(retrieved) != 3 {
		t.Errorf("expected 3 messages, got %d", len(retrieved))
	}
}

func TestStore_GetMessages_ReturnsCopy(t *testing.T) {
	s := NewStore(50, 24*time.Hour)
	sess := s.GetOrCreate("", "proj-1", "user-1")

	s.AddMessages(sess.ID, []Message{{Role: "user", Content: "original"}})

	retrieved := s.GetMessages(sess.ID, 100)
	retrieved[0].Content = "modified"

	// Should not affect stored messages
	original := s.GetMessages(sess.ID, 100)
	if original[0].Content != "original" {
		t.Errorf("expected 'original', got %q — messages not properly copied", original[0].Content)
	}
}

func TestStore_ConcurrencySafety(t *testing.T) {
	s := NewStore(100, 24*time.Hour)
	sess := s.GetOrCreate("", "proj-1", "user-1")

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				s.AddMessages(sess.ID, []Message{{Role: "user", Content: "concurrent"}})
				s.GetMessages(sess.ID, 10)
				s.GetOrCreate(sess.ID, "", "")
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestMessage_Fields(t *testing.T) {
	now := time.Now()
	m := Message{
		Role:      "assistant",
		Content:   "Hello!",
		Timestamp: now,
	}
	if m.Role != "assistant" {
		t.Errorf("unexpected role")
	}
	if m.Content != "Hello!" {
		t.Errorf("unexpected content")
	}
	if !m.Timestamp.Equal(now) {
		t.Errorf("unexpected timestamp")
	}
}

func TestSession_Fields(t *testing.T) {
	now := time.Now()
	sess := Session{
		ID:        "sess-1",
		ProjectID: "proj-1",
		UserID:    "user-1",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if sess.ID != "sess-1" {
		t.Errorf("unexpected ID")
	}
	if sess.ProjectID != "proj-1" {
		t.Errorf("unexpected project")
	}
}

func TestNewStore(t *testing.T) {
	s := NewStore(30, 1*time.Hour)
	if s == nil {
		t.Fatal("expected non-nil store")
	}
	if s.maxMsgs != 30 {
		t.Errorf("expected maxMsgs 30, got %d", s.maxMsgs)
	}
}
