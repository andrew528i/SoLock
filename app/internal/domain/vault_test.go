package domain

import "testing"

func TestAllocateSlotPopsFreeSlot(t *testing.T) {
	v := &VaultMeta{
		NextIndex: 10,
		FreeSlots: []uint32{3, 7},
	}

	if got := v.AllocateSlot(); got != 3 {
		t.Fatalf("first call: want 3, got %d", got)
	}
	if len(v.FreeSlots) != 1 || v.FreeSlots[0] != 7 {
		t.Fatalf("free slots after first pop: %v", v.FreeSlots)
	}
	if v.NextIndex != 10 {
		t.Fatalf("NextIndex should not change while FreeSlots non-empty: got %d", v.NextIndex)
	}

	if got := v.AllocateSlot(); got != 7 {
		t.Fatalf("second call: want 7, got %d", got)
	}
	if len(v.FreeSlots) != 0 {
		t.Fatalf("free slots after draining: %v", v.FreeSlots)
	}

	if got := v.AllocateSlot(); got != 10 {
		t.Fatalf("third call: want 10, got %d", got)
	}
	if v.NextIndex != 11 {
		t.Fatalf("NextIndex after pop: want 11, got %d", v.NextIndex)
	}
}

func TestAllocateGroupSlotIncrements(t *testing.T) {
	v := &VaultMeta{NextGroupIndex: 5}

	if got := v.AllocateGroupSlot(); got != 5 {
		t.Fatalf("first call: want 5, got %d", got)
	}
	if v.NextGroupIndex != 6 {
		t.Fatalf("NextGroupIndex after alloc: want 6, got %d", v.NextGroupIndex)
	}
	if got := v.AllocateGroupSlot(); got != 6 {
		t.Fatalf("second call: want 6, got %d", got)
	}
}
