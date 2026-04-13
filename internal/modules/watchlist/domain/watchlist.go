package domain

import "time"

// ─── IDs ──────────────────────────────────────────────────────────────────────

type EntryID string
type ListID string

func (id EntryID) String() string { return string(id) }
func (id ListID) String() string  { return string(id) }

// ─── Enumerations ─────────────────────────────────────────────────────────────

// WatchStatus tracks where a user is with a film.
type WatchStatus string

const (
	WatchStatusToWatch  WatchStatus = "to_watch"
	WatchStatusWatching WatchStatus = "watching"
	WatchStatusWatched  WatchStatus = "watched"
	WatchStatusDropped  WatchStatus = "dropped"
)

// Priority is the urgency flag a user attaches to a to-watch entry.
type Priority string

const (
	PriorityASAP    Priority = "asap"
	PrioritySomeday Priority = "someday"
)

// ListVisibility controls who can see a custom list.
type ListVisibility string

const (
	VisibilityPublic        ListVisibility = "public"
	VisibilityPrivate       ListVisibility = "private"
	VisibilityCollaborative ListVisibility = "collaborative"
)

// ─── Core entities ────────────────────────────────────────────────────────────

// WatchlistEntry is a single movie in a user's personal watchlist.
// One entry per (user_id, movie_id) pair — enforced by a unique constraint.
type WatchlistEntry struct {
	ID        EntryID
	UserID    string
	MovieID   string
	Status    WatchStatus
	Priority  *Priority
	CreatedAt time.Time
	UpdatedAt time.Time
}

// List is a user-curated collection of movies (e.g. "90s Heist Films").
// It is distinct from the implicit watchlist (WatchlistEntry).
type List struct {
	ID          ListID
	OwnerID     string
	Title       string
	Description string
	Visibility  ListVisibility
	CoverURL    string // user-uploaded or auto-generated collage URL
	ShareSlug   string // short URL slug, e.g. "abc123"
	ItemCount   int    // denormalized, updated on add/remove
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListItem is a movie inside a custom List, with optional per-item annotations.
type ListItem struct {
	ID        string
	ListID    ListID
	MovieID   string
	Note      string // "Great car chase in Act 2"
	SortOrder int    // drag-sort position; lower = higher in the list
	AddedAt   time.Time
}

// ListCollaborator grants a user write access to a collaborative list.
type ListCollaborator struct {
	ListID  ListID
	UserID  string
	AddedAt time.Time
}

// ─── Commands ─────────────────────────────────────────────────────────────────

type UpsertEntryCmd struct {
	UserID   string
	MovieID  string
	Status   WatchStatus
	Priority *Priority
}

type CreateListCmd struct {
	OwnerID     string
	Title       string
	Description string
	Visibility  ListVisibility
	CoverURL    string
}

type UpdateListCmd struct {
	ID          ListID
	OwnerID     string // ownership guard
	Title       *string
	Description *string
	Visibility  *ListVisibility
	CoverURL    *string
}

type AddListItemCmd struct {
	ListID    ListID
	MovieID   string
	Note      string
	SortOrder int
}

type UpdateListItemCmd struct {
	ListID    ListID
	MovieID   string
	Note      *string
	SortOrder *int
}

// ReorderItemsCmd carries the full desired order for a list so the handler
// can apply all positions in a single call.
type ReorderItemsCmd struct {
	ListID ListID
	// MovieIDs in the new desired order, index 0 = top of list.
	MovieIDs []string
}

// ─── Filter types ─────────────────────────────────────────────────────────────

type EntryFilter struct {
	UserID string
	Status *WatchStatus
	Limit  int
	Offset int
}

type ListFilter struct {
	OwnerID    string
	Visibility *ListVisibility
	Limit      int
	Offset     int
}
