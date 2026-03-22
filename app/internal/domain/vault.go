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
		return v.FreeSlots[0]
	}
	return v.NextIndex
}

func (v *VaultMeta) AllocateGroupSlot() uint32 {
	return v.NextGroupIndex
}
