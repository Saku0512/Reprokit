package runtime

import (
	"context"
	"sync"
)

type MemoryStore struct {
	mu sync.Mutex
	m  map[string]Session
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{m: map[string]Session{}} }

func (store *MemoryStore) Load(_ context.Context, id string) (Session, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	session, ok := store.m[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	return copySession(session), nil
}

func (store *MemoryStore) List(_ context.Context) ([]Session, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	sessions := make([]Session, 0, len(store.m))
	for _, session := range store.m {
		sessions = append(sessions, copySession(session))
	}
	return sessions, nil
}

func (store *MemoryStore) Save(_ context.Context, session Session) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.m[session.ID] = copySession(session)
	return nil
}
