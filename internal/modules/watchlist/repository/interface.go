package repository

import (
	"context"

	"github.com/Ab-dex/view-aura/internal/modules/watchlist/domain"
)

// WatchlistRepository manages a user's personal watch-status entries.
type WatchlistRepository interface {
	// Upsert inserts or updates the watch-status entry for (user_id, movie_id).
	Upsert(ctx context.Context, entry *domain.WatchlistEntry) (*domain.WatchlistEntry, error)

	// GetByUserAndMovie fetches a single entry. Returns nil, nil if absent.
	GetByUserAndMovie(ctx context.Context, userID, movieID string) (*domain.WatchlistEntry, error)

	// Delete removes an entry.
	Delete(ctx context.Context, userID, movieID string) error

	// List returns a user's watchlist, optionally filtered by status, paginated.
	List(ctx context.Context, filter domain.EntryFilter) ([]*domain.WatchlistEntry, int, error)

	// CountByStatus returns the number of entries per status for a user —
	// used to populate profile stats (movies watched, completion rate).
	CountByStatus(ctx context.Context, userID string) (map[domain.WatchStatus]int, error)
}

// ListRepository manages user-curated custom lists and their items.
type ListRepository interface {
	// ── List CRUD ────────────────────────────────────────────────────────────
	Create(ctx context.Context, list *domain.List) (*domain.List, error)
	GetByID(ctx context.Context, id domain.ListID) (*domain.List, error)
	GetByShareSlug(ctx context.Context, slug string) (*domain.List, error)
	Update(ctx context.Context, list *domain.List) (*domain.List, error)
	Delete(ctx context.Context, id domain.ListID, ownerID string) error
	ListByOwner(ctx context.Context, filter domain.ListFilter) ([]*domain.List, int, error)

	// ── Items ─────────────────────────────────────────────────────────────────
	AddItem(ctx context.Context, item *domain.ListItem) (*domain.ListItem, error)
	GetItem(ctx context.Context, listID domain.ListID, movieID string) (*domain.ListItem, error)
	UpdateItem(ctx context.Context, item *domain.ListItem) (*domain.ListItem, error)
	RemoveItem(ctx context.Context, listID domain.ListID, movieID string) error
	ListItems(ctx context.Context, listID domain.ListID) ([]*domain.ListItem, error)
	// ReorderItems sets SortOrder for every item in the list atomically.
	ReorderItems(ctx context.Context, listID domain.ListID, orderedMovieIDs []string) error
	// IncrementItemCount updates the denormalized item_count by delta (+1 or -1).
	IncrementItemCount(ctx context.Context, listID domain.ListID, delta int) error

	// ── Collaborators ─────────────────────────────────────────────────────────
	AddCollaborator(ctx context.Context, c *domain.ListCollaborator) error
	RemoveCollaborator(ctx context.Context, listID domain.ListID, userID string) error
	IsCollaborator(ctx context.Context, listID domain.ListID, userID string) (bool, error)
}
