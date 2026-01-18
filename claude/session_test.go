package claude

import "testing"

func TestNewSession(t *testing.T) {
	session := NewSession("session-123")
	if session == nil {
		t.Fatal("NewSession returned nil")
	}
	if session.ID != "session-123" {
		t.Errorf("ID = %q, want %q", session.ID, "session-123")
	}
	if session.IsForked {
		t.Error("IsForked should be false")
	}
	if session.ParentID != "" {
		t.Error("ParentID should be empty")
	}
}

func TestSession_Fork(t *testing.T) {
	original := NewSession("session-123")
	forked := original.Fork()

	if forked == nil {
		t.Fatal("Fork returned nil")
	}
	if forked.ParentID != "session-123" {
		t.Errorf("ParentID = %q, want %q", forked.ParentID, "session-123")
	}
	if !forked.IsForked {
		t.Error("IsForked should be true")
	}
	if forked.ID != "" {
		t.Error("ID should be empty (assigned by CLI)")
	}
}

func TestNewFileCheckpoint(t *testing.T) {
	cp := NewFileCheckpoint("msg-456", "session-123")
	if cp == nil {
		t.Fatal("NewFileCheckpoint returned nil")
	}
	if cp.UserMessageID != "msg-456" {
		t.Errorf("UserMessageID = %q, want %q", cp.UserMessageID, "msg-456")
	}
	if cp.SessionID != "session-123" {
		t.Errorf("SessionID = %q, want %q", cp.SessionID, "session-123")
	}
}
