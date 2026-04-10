package domain

type VaultMeta struct {
	NextIndex      uint32
	EntryCount     uint32
	FreeSlots      []uint32
	NextGroupIndex uint32
	GroupCount     uint32
}

func (v *VaultMeta) AllocateSlot() uint32 {
	if len(v.FreeSlots) > 0 {
		slot := v.FreeSlots[0]
		v.FreeSlots = v.FreeSlots[1:]
		return slot
	}
	slot := v.NextIndex
	v.NextIndex++
	return slot
}

func (v *VaultMeta) AllocateGroupSlot() uint32 {
	slot := v.NextGroupIndex
	v.NextGroupIndex++
	return slot
}
