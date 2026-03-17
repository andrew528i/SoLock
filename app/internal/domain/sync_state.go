package domain

import "time"

type SyncState struct {
	NextIndex  uint32
	EntryCount uint32
	LastSyncAt time.Time
}
