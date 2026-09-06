package github

import "testing"

func TestAppendStripMeta_RoundTrip(t *testing.T) {
	meta := metaBlock{AtamaID: "atm-123", DependsOn: []string{"atm-1", "atm-2"}, GoalIDs: []string{"atm-g-1"}}
	body := appendMeta("hello world", meta)

	clean, got := stripMeta(body)
	if clean != "hello world" {
		t.Errorf("stripMeta() clean = %q, want %q", clean, "hello world")
	}
	if got == nil {
		t.Fatal("stripMeta() meta = nil, want non-nil")
	}
	if got.AtamaID != meta.AtamaID {
		t.Errorf("AtamaID = %q, want %q", got.AtamaID, meta.AtamaID)
	}
	if len(got.DependsOn) != 2 || got.DependsOn[0] != "atm-1" {
		t.Errorf("DependsOn = %v, want [atm-1 atm-2]", got.DependsOn)
	}
	if len(got.GoalIDs) != 1 || got.GoalIDs[0] != "atm-g-1" {
		t.Errorf("GoalIDs = %v, want [atm-g-1]", got.GoalIDs)
	}
}

func TestStripMeta_NoBlock(t *testing.T) {
	clean, meta := stripMeta("plain body, no metadata here")
	if clean != "plain body, no metadata here" {
		t.Errorf("stripMeta() clean = %q, want unchanged", clean)
	}
	if meta != nil {
		t.Errorf("stripMeta() meta = %+v, want nil", meta)
	}
}

func TestAppendMeta_ReplacesExistingBlock(t *testing.T) {
	first := appendMeta("body", metaBlock{AtamaID: "atm-1", DependsOn: []string{"atm-x"}})
	second := appendMeta(first, metaBlock{AtamaID: "atm-1", DependsOn: []string{"atm-y"}})

	clean, meta := stripMeta(second)
	if clean != "body" {
		t.Errorf("clean = %q, want %q", clean, "body")
	}
	if len(meta.DependsOn) != 1 || meta.DependsOn[0] != "atm-y" {
		t.Errorf("DependsOn = %v, want [atm-y] (stale block should be replaced, not duplicated)", meta.DependsOn)
	}
}
