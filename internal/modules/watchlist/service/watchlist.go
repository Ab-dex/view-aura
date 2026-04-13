package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strings"

	"github.com/google/uuid"

	"github.com/Ab-dex/view-aura/internal/modules/watchlist/domain"
	"github.com/Ab-dex/view-aura/internal/modules/watchlist/repository"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

// WatchlistService defines all business operations for the watchlist module.
type WatchlistService interface {
	// ── Personal watchlist ────────────────────────────────────────────────────
	UpsertEntry(ctx context.Context, cmd domain.UpsertEntryCmd) (*domain.WatchlistEntry, error)
	DeleteEntry(ctx context.Context, userID, movieID string) error
	GetEntry(ctx context.Context, userID, movieID string) (*domain.WatchlistEntry, error)
	ListEntries(ctx context.Context, filter domain.EntryFilter) ([]*domain.WatchlistEntry, int, error)
	GetStatusCounts(ctx context.Context, userID string) (map[domain.WatchStatus]int, error)

	// ── Custom lists ──────────────────────────────────────────────────────────
	CreateList(ctx context.Context, cmd domain.CreateListCmd) (*domain.List, error)
	GetList(ctx context.Context, id domain.ListID, callerID string) (*ListWithItems, error)
	GetListBySlug(ctx context.Context, slug string, callerID string) (*ListWithItems, error)
	UpdateList(ctx context.Context, cmd domain.UpdateListCmd) (*domain.List, error)
	DeleteList(ctx context.Context, id domain.ListID, ownerID string) error
	MyLists(ctx context.Context, filter domain.ListFilter) ([]*domain.List, int, error)

	// ── List items ────────────────────────────────────────────────────────────
	AddItem(ctx context.Context, cmd domain.AddListItemCmd, callerID string) (*domain.ListItem, error)
	UpdateItem(ctx context.Context, cmd domain.UpdateListItemCmd, callerID string) (*domain.ListItem, error)
	RemoveItem(ctx context.Context, listID domain.ListID, movieID string, callerID string) error
	ReorderItems(ctx context.Context, cmd domain.ReorderItemsCmd, callerID string) error

	// ── Collaborators ─────────────────────────────────────────────────────────
	AddCollaborator(ctx context.Context, listID domain.ListID, ownerID, targetUserID string) error
	RemoveCollaborator(ctx context.Context, listID domain.ListID, ownerID, targetUserID string) error
}

// ListWithItems is the enriched view returned on list GET.
type ListWithItems struct {
	List  *domain.List
	Items []*domain.ListItem
}

type watchlistService struct {
	watchlists repository.WatchlistRepository
	lists      repository.ListRepository
}

func NewWatchlistService(
	watchlists repository.WatchlistRepository,
	lists repository.ListRepository,
) WatchlistService {
	return &watchlistService{watchlists: watchlists, lists: lists}
}

// ─── Personal watchlist ───────────────────────────────────────────────────────

func (s *watchlistService) UpsertEntry(ctx context.Context, cmd domain.UpsertEntryCmd) (*domain.WatchlistEntry, error) {
	if err := validateEntry(cmd); err != nil {
		return nil, err
	}

	entry := &domain.WatchlistEntry{
		ID:       domain.EntryID(uuid.New().String()),
		UserID:   cmd.UserID,
		MovieID:  cmd.MovieID,
		Status:   cmd.Status,
		Priority: cmd.Priority,
	}

	result, err := s.watchlists.Upsert(ctx, entry)
	if err != nil {
		return nil, err
	}

	logger.FromContext(ctx).Info().
		Str("user_id", cmd.UserID).
		Str("movie_id", cmd.MovieID).
		Str("status", string(cmd.Status)).
		Msg("watchlist entry upserted")

	return result, nil
}

func (s *watchlistService) DeleteEntry(ctx context.Context, userID, movieID string) error {
	return s.watchlists.Delete(ctx, userID, movieID)
}

func (s *watchlistService) GetEntry(ctx context.Context, userID, movieID string) (*domain.WatchlistEntry, error) {
	return s.watchlists.GetByUserAndMovie(ctx, userID, movieID)
}

func (s *watchlistService) ListEntries(ctx context.Context, filter domain.EntryFilter) ([]*domain.WatchlistEntry, int, error) {
	return s.watchlists.List(ctx, filter)
}

func (s *watchlistService) GetStatusCounts(ctx context.Context, userID string) (map[domain.WatchStatus]int, error) {
	return s.watchlists.CountByStatus(ctx, userID)
}

// ─── Custom lists ─────────────────────────────────────────────────────────────

func (s *watchlistService) CreateList(ctx context.Context, cmd domain.CreateListCmd) (*domain.List, error) {
	if strings.TrimSpace(cmd.Title) == "" {
		return nil, apierror.Validation("list title is required", nil)
	}

	list := &domain.List{
		ID:          domain.ListID(uuid.New().String()),
		OwnerID:     cmd.OwnerID,
		Title:       strings.TrimSpace(cmd.Title),
		Description: strings.TrimSpace(cmd.Description),
		Visibility:  cmd.Visibility,
		CoverURL:    cmd.CoverURL,
		ShareSlug:   generateShareSlug(),
	}

	return s.lists.Create(ctx, list)
}

func (s *watchlistService) GetList(ctx context.Context, id domain.ListID, callerID string) (*ListWithItems, error) {
	list, err := s.lists.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.authorizeAndEnrich(ctx, list, callerID)
}

func (s *watchlistService) GetListBySlug(ctx context.Context, slug string, callerID string) (*ListWithItems, error) {
	list, err := s.lists.GetByShareSlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	return s.authorizeAndEnrich(ctx, list, callerID)
}

// authorizeAndEnrich checks read access then fetches items.
func (s *watchlistService) authorizeAndEnrich(ctx context.Context, list *domain.List, callerID string) (*ListWithItems, error) {
	if err := s.assertReadAccess(ctx, list, callerID); err != nil {
		return nil, err
	}
	items, err := s.lists.ListItems(ctx, list.ID)
	if err != nil {
		return nil, err
	}
	return &ListWithItems{List: list, Items: items}, nil
}

func (s *watchlistService) UpdateList(ctx context.Context, cmd domain.UpdateListCmd) (*domain.List, error) {
	list, err := s.lists.GetByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}
	if list.OwnerID != cmd.OwnerID {
		return nil, apierror.Forbidden("only the list owner can edit it")
	}

	if cmd.Title != nil {
		list.Title = strings.TrimSpace(*cmd.Title)
	}
	if cmd.Description != nil {
		list.Description = strings.TrimSpace(*cmd.Description)
	}
	if cmd.Visibility != nil {
		list.Visibility = *cmd.Visibility
	}
	if cmd.CoverURL != nil {
		list.CoverURL = *cmd.CoverURL
	}

	return s.lists.Update(ctx, list)
}

func (s *watchlistService) DeleteList(ctx context.Context, id domain.ListID, ownerID string) error {
	return s.lists.Delete(ctx, id, ownerID)
}

func (s *watchlistService) MyLists(ctx context.Context, filter domain.ListFilter) ([]*domain.List, int, error) {
	return s.lists.ListByOwner(ctx, filter)
}

// ─── List items ───────────────────────────────────────────────────────────────

func (s *watchlistService) AddItem(ctx context.Context, cmd domain.AddListItemCmd, callerID string) (*domain.ListItem, error) {
	if err := s.assertWriteAccess(ctx, cmd.ListID, callerID); err != nil {
		return nil, err
	}

	item := &domain.ListItem{
		ID:        uuid.New().String(),
		ListID:    cmd.ListID,
		MovieID:   cmd.MovieID,
		Note:      strings.TrimSpace(cmd.Note),
		SortOrder: cmd.SortOrder,
	}

	result, err := s.lists.AddItem(ctx, item)
	if err != nil {
		return nil, err
	}

	// Increment the denormalized item count only if a new row was inserted.
	// AddItem returns the existing item on conflict, so compare IDs.
	if result.ID == item.ID {
		_ = s.lists.IncrementItemCount(ctx, cmd.ListID, 1)
	}

	return result, nil
}

func (s *watchlistService) UpdateItem(ctx context.Context, cmd domain.UpdateListItemCmd, callerID string) (*domain.ListItem, error) {
	if err := s.assertWriteAccess(ctx, cmd.ListID, callerID); err != nil {
		return nil, err
	}

	item, err := s.lists.GetItem(ctx, cmd.ListID, cmd.MovieID)
	if err != nil {
		return nil, err
	}

	if cmd.Note != nil {
		item.Note = strings.TrimSpace(*cmd.Note)
	}
	if cmd.SortOrder != nil {
		item.SortOrder = *cmd.SortOrder
	}

	return s.lists.UpdateItem(ctx, item)
}

func (s *watchlistService) RemoveItem(ctx context.Context, listID domain.ListID, movieID string, callerID string) error {
	if err := s.assertWriteAccess(ctx, listID, callerID); err != nil {
		return err
	}
	if err := s.lists.RemoveItem(ctx, listID, movieID); err != nil {
		return err
	}
	return s.lists.IncrementItemCount(ctx, listID, -1)
}

func (s *watchlistService) ReorderItems(ctx context.Context, cmd domain.ReorderItemsCmd, callerID string) error {
	if err := s.assertWriteAccess(ctx, cmd.ListID, callerID); err != nil {
		return err
	}
	return s.lists.ReorderItems(ctx, cmd.ListID, cmd.MovieIDs)
}

// ─── Collaborators ────────────────────────────────────────────────────────────

func (s *watchlistService) AddCollaborator(ctx context.Context, listID domain.ListID, ownerID, targetUserID string) error {
	list, err := s.lists.GetByID(ctx, listID)
	if err != nil {
		return err
	}
	if list.OwnerID != ownerID {
		return apierror.Forbidden("only the list owner can add collaborators")
	}
	if list.Visibility != domain.VisibilityCollaborative {
		return apierror.Validation("list must be collaborative to add collaborators", nil)
	}
	return s.lists.AddCollaborator(ctx, &domain.ListCollaborator{ListID: listID, UserID: targetUserID})
}

func (s *watchlistService) RemoveCollaborator(ctx context.Context, listID domain.ListID, ownerID, targetUserID string) error {
	list, err := s.lists.GetByID(ctx, listID)
	if err != nil {
		return err
	}
	if list.OwnerID != ownerID {
		return apierror.Forbidden("only the list owner can remove collaborators")
	}
	return s.lists.RemoveCollaborator(ctx, listID, targetUserID)
}

// ─── Access helpers ───────────────────────────────────────────────────────────

// assertReadAccess allows: owner, collaborators, or the list is public.
func (s *watchlistService) assertReadAccess(ctx context.Context, list *domain.List, callerID string) error {
	if list.Visibility == domain.VisibilityPublic || list.Visibility == domain.VisibilityCollaborative {
		return nil
	}
	// Private list — must be the owner.
	if list.OwnerID == callerID {
		return nil
	}
	return apierror.Forbidden("this list is private")
}

// assertWriteAccess allows: owner or collaborators on a collaborative list.
func (s *watchlistService) assertWriteAccess(ctx context.Context, listID domain.ListID, callerID string) error {
	list, err := s.lists.GetByID(ctx, listID)
	if err != nil {
		return err
	}
	if list.OwnerID == callerID {
		return nil
	}
	if list.Visibility == domain.VisibilityCollaborative {
		ok, err := s.lists.IsCollaborator(ctx, listID, callerID)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return apierror.Forbidden("you do not have write access to this list")
}

// ─── Validation ───────────────────────────────────────────────────────────────

func validateEntry(cmd domain.UpsertEntryCmd) error {
	switch cmd.Status {
	case domain.WatchStatusToWatch, domain.WatchStatusWatching,
		domain.WatchStatusWatched, domain.WatchStatusDropped:
	default:
		return apierror.Validation("invalid watch status", nil)
	}
	if cmd.UserID == "" || cmd.MovieID == "" {
		return apierror.Validation("user_id and movie_id are required", nil)
	}
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// generateShareSlug produces an 8-character URL-safe random string.
func generateShareSlug() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)[:8]
}
