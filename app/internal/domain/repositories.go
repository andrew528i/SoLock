package domain

import "context"

type EntryRepository interface {
	Save(ctx context.Context, entry *Entry) error
	Get(ctx context.Context, id string) (*Entry, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]*Entry, error)
	Count(ctx context.Context) (int, error)
	MarkSynced(ctx context.Context, id string) error
	ClearAll(ctx context.Context) error
}

type GroupRepository interface {
	Save(ctx context.Context, group *Group) error
	Get(ctx context.Context, index uint32) (*Group, error)
	Delete(ctx context.Context, index uint32) error
	List(ctx context.Context) ([]*Group, error)
	ClearAll(ctx context.Context) error
}

type ConfigRepository interface {
	Set(ctx context.Context, key, value string) error
	Get(ctx context.Context, key string) (string, error)
}

type SyncStateRepository interface {
	Get(ctx context.Context) (*SyncState, error)
	Set(ctx context.Context, state *SyncState) error
}

type VaultRepository interface {
	Initialize(ctx context.Context) error
	Reset(ctx context.Context) error
	Exists(ctx context.Context) (bool, error)
	GetMeta(ctx context.Context) (*VaultMeta, error)
	AddEntry(ctx context.Context, index uint32, data []byte) error
	UpdateEntry(ctx context.Context, index uint32, data []byte, expectedUpdatedAt int64) error
	DeleteEntry(ctx context.Context, index uint32) error
	GetEntry(ctx context.Context, index uint32) (*EntryAccount, error)
	GetEntriesBatch(ctx context.Context, indices []uint32) (map[uint32]*EntryAccount, error)
	AddGroup(ctx context.Context, index uint32, data []byte) error
	UpdateGroup(ctx context.Context, index uint32, data []byte) error
	DeleteGroup(ctx context.Context, index uint32) error
	PurgeGroup(ctx context.Context, index uint32) error
	GetGroup(ctx context.Context, index uint32) (*GroupAccount, error)
	GetGroupsBatch(ctx context.Context, indices []uint32) (map[uint32]*GroupAccount, error)
	IsProgramDeployed(ctx context.Context) (bool, error)
	GetBalance(ctx context.Context) (uint64, error)
	DeployProgram(ctx context.Context, programBinary []byte) error
	CloseProgram(ctx context.Context) error
}
