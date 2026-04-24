package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Ab-dex/view-aura/internal/modules/auth/domain"
)

type inMemoryOAuthStateRepo struct {
	mu    sync.Mutex
	store map[string]memEntry
}

type memEntry struct {
	value     []byte
	expiresAt time.Time
}

func NewInMemoryOAuthStateRepository() OAuthStateRepository {
	return &inMemoryOAuthStateRepo{
		store: make(map[string]memEntry),
	}
}

func (r *inMemoryOAuthStateRepo) Save(ctx context.Context, state *domain.OAuthState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("auth: marshal oauth state: %w", err)
	}

	ttl := time.Until(state.ExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("auth: oauth state already expired")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.store[state.State] = memEntry{
		value:     data,
		expiresAt: state.ExpiresAt,
	}

	return nil
}

func (r *inMemoryOAuthStateRepo) GetAndDelete(ctx context.Context, state string) (*domain.OAuthState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.store[state]
	if !ok {
		return nil, nil
	}

	// expired
	if time.Now().After(entry.expiresAt) {
		delete(r.store, state)
		return nil, nil
	}

	// delete-on-read (like Redis Lua script)
	delete(r.store, state)

	var s domain.OAuthState
	if err := json.Unmarshal(entry.value, &s); err != nil {
		return nil, fmt.Errorf("auth: unmarshal oauth state: %w", err)
	}

	if s.IsExpired() {
		return nil, nil
	}

	return &s, nil
}

type InMemoryMFAChallengeRepo struct {
	mu    sync.Mutex
	store map[string]memChallenge
}

type memChallenge struct {
	data      []byte
	expiresAt time.Time
}

func NewInMemoryMFAChallengeRepo() MfaChallengeStore {
	return &InMemoryMFAChallengeRepo{
		store: make(map[string]memChallenge),
	}
}

func (r *InMemoryMFAChallengeRepo) CreateChallenge(ctx context.Context, ch *domain.MFAChallenge) error {
	data, err := json.Marshal(ch)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.store[ch.ID] = memChallenge{
		data:      data,
		expiresAt: ch.ExpiresAt,
	}

	return nil
}

func (r *InMemoryMFAChallengeRepo) GetChallenge(ctx context.Context, id string) (*domain.MFAChallenge, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.store[id]
	if !ok {
		return nil, nil
	}

	if time.Now().After(entry.expiresAt) {
		delete(r.store, id)
		return nil, nil
	}

	var ch domain.MFAChallenge
	if err := json.Unmarshal(entry.data, &ch); err != nil {
		return nil, err
	}

	return &ch, nil
}

func (r *InMemoryMFAChallengeRepo) DeleteChallenge(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.store, id)
	return nil
}
