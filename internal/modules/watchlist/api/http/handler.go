package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Ab-dex/view-aura/internal/modules/watchlist/domain"
	"github.com/Ab-dex/view-aura/internal/modules/watchlist/service"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

type WatchlistHandler struct {
	svc service.WatchlistService
}

func NewWatchlistHandler(svc service.WatchlistService) *WatchlistHandler {
	return &WatchlistHandler{svc: svc}
}

// Route layout — all protected (auth required), grouped under /api/v1:
//
//	Personal watchlist:
//	  GET    /me/watchlist               — list entries (filter by ?status=)
//	  PUT    /me/watchlist/:movie_id     — upsert entry
//	  GET    /me/watchlist/:movie_id     — get single entry
//	  DELETE /me/watchlist/:movie_id     — remove entry
//	  GET    /me/watchlist/stats         — status counts
//
//	Custom lists:
//	  GET    /lists/:id_or_slug          — get list + items (public/private check)
//	  GET    /me/lists                   — my lists
//	  POST   /me/lists                   — create list
//	  PATCH  /me/lists/:id              — update list
//	  DELETE /me/lists/:id              — delete list
//	  POST   /me/lists/:id/items         — add item
//	  PATCH  /me/lists/:id/items/:movie_id — update item note/order
//	  DELETE /me/lists/:id/items/:movie_id — remove item
//	  PUT    /me/lists/:id/items/order   — reorder (full order in body)
//	  POST   /me/lists/:id/collaborators — add collaborator
//	  DELETE /me/lists/:id/collaborators/:user_id — remove collaborator

func (h *WatchlistHandler) RegisterPublicRoutes(r gin.IRouter) {
	r.GET("/lists/:id_or_slug", h.GetList)
}

func (h *WatchlistHandler) RegisterProtectedRoutes(r gin.IRouter) {
	// Personal watchlist
	r.GET("/me/watchlist", h.ListEntries)
	r.PUT("/me/watchlist/:movie_id", h.UpsertEntry)
	r.GET("/me/watchlist/:movie_id", h.GetEntry)
	r.DELETE("/me/watchlist/:movie_id", h.DeleteEntry)
	r.GET("/me/watchlist/stats", h.WatchlistStats)

	// Custom lists
	r.GET("/me/lists", h.MyLists)
	r.POST("/me/lists", h.CreateList)
	r.PATCH("/me/lists/:id", h.UpdateList)
	r.DELETE("/me/lists/:id", h.DeleteList)
	r.POST("/me/lists/:id/items", h.AddItem)
	r.PATCH("/me/lists/:id/items/:movie_id", h.UpdateItem)
	r.DELETE("/me/lists/:id/items/:movie_id", h.RemoveItem)
	r.PUT("/me/lists/:id/items/order", h.ReorderItems)
	r.POST("/me/lists/:id/collaborators", h.AddCollaborator)
	r.DELETE("/me/lists/:id/collaborators/:user_id", h.RemoveCollaborator)
}

// ─── DTOs ─────────────────────────────────────────────────────────────────────

type upsertEntryRequest struct {
	Status   string  `json:"status"   binding:"required"`
	Priority *string `json:"priority"`
}

type createListRequest struct {
	Title       string `json:"title"       binding:"required"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"  binding:"required"`
	CoverURL    string `json:"cover_url"`
}

type updateListRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Visibility  *string `json:"visibility"`
	CoverURL    *string `json:"cover_url"`
}

type addItemRequest struct {
	MovieID   string `json:"movie_id"   binding:"required"`
	Note      string `json:"note"`
	SortOrder int    `json:"sort_order"`
}

type updateItemRequest struct {
	Note      *string `json:"note"`
	SortOrder *int    `json:"sort_order"`
}

type reorderRequest struct {
	MovieIDs []string `json:"movie_ids" binding:"required"`
}

type collaboratorRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

// ─── Response types ───────────────────────────────────────────────────────────

type entryResponse struct {
	ID        string  `json:"id"`
	MovieID   string  `json:"movie_id"`
	Status    string  `json:"status"`
	Priority  *string `json:"priority,omitempty"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type listResponse struct {
	ID          string `json:"id"`
	OwnerID     string `json:"owner_id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Visibility  string `json:"visibility"`
	CoverURL    string `json:"cover_url,omitempty"`
	ShareSlug   string `json:"share_slug"`
	ShareURL    string `json:"share_url"`
	ItemCount   int    `json:"item_count"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type listDetailResponse struct {
	listResponse
	Items []itemResponse `json:"items"`
}

type itemResponse struct {
	ID        string `json:"id"`
	MovieID   string `json:"movie_id"`
	Note      string `json:"note,omitempty"`
	SortOrder int    `json:"sort_order"`
	AddedAt   string `json:"added_at"`
}

// ─── Handlers — personal watchlist ───────────────────────────────────────────

func (h *WatchlistHandler) UpsertEntry(c *gin.Context) {
	var req upsertEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	cmd := domain.UpsertEntryCmd{
		UserID:  mustUserID(c),
		MovieID: c.Param("movie_id"),
		Status:  domain.WatchStatus(req.Status),
	}
	if req.Priority != nil {
		p := domain.Priority(*req.Priority)
		cmd.Priority = &p
	}

	entry, err := h.svc.UpsertEntry(c.Request.Context(), cmd)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toEntryResponse(entry))
}

func (h *WatchlistHandler) DeleteEntry(c *gin.Context) {
	if err := h.svc.DeleteEntry(c.Request.Context(), mustUserID(c), c.Param("movie_id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *WatchlistHandler) GetEntry(c *gin.Context) {
	entry, err := h.svc.GetEntry(c.Request.Context(), mustUserID(c), c.Param("movie_id"))
	if err != nil {
		respondError(c, err)
		return
	}
	if entry == nil {
		c.JSON(http.StatusOK, gin.H{"entry": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"entry": toEntryResponse(entry)})
}

func (h *WatchlistHandler) ListEntries(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filter := domain.EntryFilter{
		UserID: mustUserID(c),
		Limit:  limit,
		Offset: offset,
	}
	if s := c.Query("status"); s != "" {
		st := domain.WatchStatus(s)
		filter.Status = &st
	}

	entries, total, err := h.svc.ListEntries(c.Request.Context(), filter)
	if err != nil {
		respondError(c, err)
		return
	}

	out := make([]entryResponse, 0, len(entries))
	for _, e := range entries {
		out = append(out, toEntryResponse(e))
	}
	c.JSON(http.StatusOK, gin.H{
		"entries": out,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (h *WatchlistHandler) WatchlistStats(c *gin.Context) {
	counts, err := h.svc.GetStatusCounts(c.Request.Context(), mustUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	// Convert map keys to strings for JSON serialization.
	out := make(map[string]int, len(counts))
	for k, v := range counts {
		out[string(k)] = v
	}
	c.JSON(http.StatusOK, gin.H{"stats": out})
}

// ─── Handlers — custom lists ──────────────────────────────────────────────────

func (h *WatchlistHandler) GetList(c *gin.Context) {
	idOrSlug := c.Param("id_or_slug")
	callerID, _ := c.Get("user_id") // may be empty for unauthenticated callers
	caller := ""
	if callerID != nil {
		caller = callerID.(string)
	}

	var (
		result *service.ListWithItems
		err    error
	)
	if len(idOrSlug) == 36 {
		result, err = h.svc.GetList(c.Request.Context(), domain.ListID(idOrSlug), caller)
	} else {
		result, err = h.svc.GetListBySlug(c.Request.Context(), idOrSlug, caller)
	}
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toListDetailResponse(result))
}

func (h *WatchlistHandler) MyLists(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	lists, total, err := h.svc.MyLists(c.Request.Context(), domain.ListFilter{
		OwnerID: mustUserID(c),
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]listResponse, 0, len(lists))
	for _, l := range lists {
		out = append(out, toListResponse(l))
	}
	c.JSON(http.StatusOK, gin.H{"lists": out, "total": total, "limit": limit, "offset": offset})
}

func (h *WatchlistHandler) CreateList(c *gin.Context) {
	var req createListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	list, err := h.svc.CreateList(c.Request.Context(), domain.CreateListCmd{
		OwnerID:     mustUserID(c),
		Title:       req.Title,
		Description: req.Description,
		Visibility:  domain.ListVisibility(req.Visibility),
		CoverURL:    req.CoverURL,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toListResponse(list))
}

func (h *WatchlistHandler) UpdateList(c *gin.Context) {
	var req updateListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	cmd := domain.UpdateListCmd{
		ID:          domain.ListID(c.Param("id")),
		OwnerID:     mustUserID(c),
		Title:       req.Title,
		Description: req.Description,
		CoverURL:    req.CoverURL,
	}
	if req.Visibility != nil {
		v := domain.ListVisibility(*req.Visibility)
		cmd.Visibility = &v
	}

	list, err := h.svc.UpdateList(c.Request.Context(), cmd)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toListResponse(list))
}

func (h *WatchlistHandler) DeleteList(c *gin.Context) {
	if err := h.svc.DeleteList(c.Request.Context(), domain.ListID(c.Param("id")), mustUserID(c)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── Handlers — list items ────────────────────────────────────────────────────

func (h *WatchlistHandler) AddItem(c *gin.Context) {
	var req addItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	item, err := h.svc.AddItem(c.Request.Context(), domain.AddListItemCmd{
		ListID:    domain.ListID(c.Param("id")),
		MovieID:   req.MovieID,
		Note:      req.Note,
		SortOrder: req.SortOrder,
	}, mustUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toItemResponse(item))
}

func (h *WatchlistHandler) UpdateItem(c *gin.Context) {
	var req updateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	item, err := h.svc.UpdateItem(c.Request.Context(), domain.UpdateListItemCmd{
		ListID:    domain.ListID(c.Param("id")),
		MovieID:   c.Param("movie_id"),
		Note:      req.Note,
		SortOrder: req.SortOrder,
	}, mustUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toItemResponse(item))
}

func (h *WatchlistHandler) RemoveItem(c *gin.Context) {
	if err := h.svc.RemoveItem(
		c.Request.Context(),
		domain.ListID(c.Param("id")),
		c.Param("movie_id"),
		mustUserID(c),
	); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *WatchlistHandler) ReorderItems(c *gin.Context) {
	var req reorderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	if err := h.svc.ReorderItems(c.Request.Context(), domain.ReorderItemsCmd{
		ListID:   domain.ListID(c.Param("id")),
		MovieIDs: req.MovieIDs,
	}, mustUserID(c)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── Handlers — collaborators ─────────────────────────────────────────────────

func (h *WatchlistHandler) AddCollaborator(c *gin.Context) {
	var req collaboratorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	if err := h.svc.AddCollaborator(
		c.Request.Context(),
		domain.ListID(c.Param("id")),
		mustUserID(c),
		req.UserID,
	); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *WatchlistHandler) RemoveCollaborator(c *gin.Context) {
	if err := h.svc.RemoveCollaborator(
		c.Request.Context(),
		domain.ListID(c.Param("id")),
		mustUserID(c),
		c.Param("user_id"),
	); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── Response mappers ─────────────────────────────────────────────────────────

func toEntryResponse(e *domain.WatchlistEntry) entryResponse {
	resp := entryResponse{
		ID:        string(e.ID),
		MovieID:   e.MovieID,
		Status:    string(e.Status),
		CreatedAt: e.CreatedAt.Format(time.RFC3339),
		UpdatedAt: e.UpdatedAt.Format(time.RFC3339),
	}
	if e.Priority != nil {
		p := string(*e.Priority)
		resp.Priority = &p
	}
	return resp
}

func toListResponse(l *domain.List) listResponse {
	return listResponse{
		ID:          string(l.ID),
		OwnerID:     l.OwnerID,
		Title:       l.Title,
		Description: l.Description,
		Visibility:  string(l.Visibility),
		CoverURL:    l.CoverURL,
		ShareSlug:   l.ShareSlug,
		ShareURL:    "/lists/" + l.ShareSlug,
		ItemCount:   l.ItemCount,
		CreatedAt:   l.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   l.UpdatedAt.Format(time.RFC3339),
	}
}

func toListDetailResponse(r *service.ListWithItems) listDetailResponse {
	items := make([]itemResponse, 0, len(r.Items))
	for _, item := range r.Items {
		items = append(items, toItemResponse(item))
	}
	return listDetailResponse{
		listResponse: toListResponse(r.List),
		Items:        items,
	}
}

func toItemResponse(item *domain.ListItem) itemResponse {
	return itemResponse{
		ID:        item.ID,
		MovieID:   item.MovieID,
		Note:      item.Note,
		SortOrder: item.SortOrder,
		AddedAt:   item.AddedAt.Format(time.RFC3339),
	}
}

// ─── Shared helpers ───────────────────────────────────────────────────────────

func mustUserID(c *gin.Context) string {
	v, _ := c.Get("user_id")
	return v.(string)
}

func respondError(c *gin.Context, err error) {
	log := logger.FromContext(c.Request.Context())
	ae, ok := apierror.As(err)
	if !ok {
		ae = apierror.Internal("unexpected error", err)
	}
	if ae.HTTPStatus >= 500 {
		log.Error().Err(err).Str("code", string(ae.Code)).Msg("internal error")
	}
	c.JSON(ae.HTTPStatus, gin.H{"error": gin.H{
		"code": ae.Code, "message": ae.Message, "details": ae.Details,
	}})
}
