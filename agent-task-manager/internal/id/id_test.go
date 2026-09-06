package id

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestNewTaskFormat(t *testing.T) {
	got := NewTask()
	if !strings.HasPrefix(got, TaskPrefix) {
		t.Fatalf("NewTask() = %q, want prefix %q", got, TaskPrefix)
	}
	checkUUIDv8(t, strings.TrimPrefix(got, TaskPrefix))
}

func TestNewGoalFormat(t *testing.T) {
	got := NewGoal()
	if !strings.HasPrefix(got, GoalPrefix) {
		t.Fatalf("NewGoal() = %q, want prefix %q", got, GoalPrefix)
	}
	checkUUIDv8(t, strings.TrimPrefix(got, GoalPrefix))
}

func TestNewCommentFormat(t *testing.T) {
	got := NewComment()
	if !strings.HasPrefix(got, CommentPrefix) {
		t.Fatalf("NewComment() = %q, want prefix %q", got, CommentPrefix)
	}
	checkUUIDv8(t, strings.TrimPrefix(got, CommentPrefix))
}

// checkUUIDv8 validates the RFC 9562 shape: 8-4-4-4-12 hex groups, version
// nibble 8, and variant bits 10.
func checkUUIDv8(t *testing.T, uuid string) {
	t.Helper()

	parts := strings.Split(uuid, "-")
	if len(parts) != 5 {
		t.Fatalf("uuid %q: want 5 hyphen-separated groups, got %d", uuid, len(parts))
	}
	lens := []int{8, 4, 4, 4, 12}
	for i, p := range parts {
		if len(p) != lens[i] {
			t.Fatalf("uuid %q: group %d has length %d, want %d", uuid, i, len(p), lens[i])
		}
	}

	raw := strings.ReplaceAll(uuid, "-", "")
	b, err := hex.DecodeString(raw)
	if err != nil {
		t.Fatalf("uuid %q: not valid hex: %v", uuid, err)
	}
	if len(b) != 16 {
		t.Fatalf("uuid %q: decoded to %d bytes, want 16", uuid, len(b))
	}

	if version := b[6] >> 4; version != 0x8 {
		t.Errorf("uuid %q: version nibble = %x, want 8", uuid, version)
	}
	if variant := b[8] >> 6; variant != 0b10 {
		t.Errorf("uuid %q: variant bits = %b, want 10", uuid, variant)
	}
}

func TestNewTaskIsTimeSortable(t *testing.T) {
	orig := nowMilli
	defer func() { nowMilli = orig }()

	times := []int64{1000, 1000, 2000, 3000}
	var ids []string
	for _, ms := range times {
		nowMilli = func() int64 { return ms }
		ids = append(ids, NewTask())
	}

	// The millisecond timestamp occupies the first 12 hex chars after the
	// prefix; lexicographic order over that slice must match creation order
	// for distinct timestamps (indices 1..3, since 0 and 1 share a millisecond).
	for i := 1; i < len(ids)-1; i++ {
		a := ids[i][:len(TaskPrefix)+12]
		b := ids[i+1][:len(TaskPrefix)+12]
		if a >= b {
			t.Errorf("ids not time-sortable: %q should sort before %q", a, b)
		}
	}
}

func TestNewTaskUnique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		got := NewTask()
		if seen[got] {
			t.Fatalf("duplicate id generated: %q", got)
		}
		seen[got] = true
	}
}
