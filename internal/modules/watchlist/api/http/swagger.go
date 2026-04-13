package httpapi

// UpsertEntry godoc
//
//	@Summary     Add / update watchlist entry
//	@Description Sets the watch status for a movie. Creates the entry if it does
//	             not exist; updates it otherwise. Status must be one of:
//	             to_watch, watching, watched, dropped.
//	             Priority must be asap or someday (optional).
//	@Tags        Watchlist
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       movie_id path string true "Movie UUID"
//	@Param       body body upsertEntryRequest true "Status and optional priority"
//	@Success     200 {object} entryResponse
//	@Failure     400 {object} errorResponse
//	@Failure     401 {object} errorResponse
//	@Router      /me/watchlist/{movie_id} [put]
func (h *WatchlistHandler) swaggerUpsertEntry() {}

// DeleteEntry godoc
//
//	@Summary     Remove watchlist entry
//	@Description Removes a movie from the user's watchlist entirely.
//	@Tags        Watchlist
//	@Security    BearerAuth
//	@Param       movie_id path string true "Movie UUID"
//	@Success     204 "Removed"
//	@Failure     401 {object} errorResponse
//	@Failure     404 {object} errorResponse
//	@Router      /me/watchlist/{movie_id} [delete]
func (h *WatchlistHandler) swaggerDeleteEntry() {}

// GetEntry godoc
//
//	@Summary     Get watchlist entry
//	@Description Returns the user's watch status for a specific movie,
//	             or `{"entry": null}` if not on their list.
//	@Tags        Watchlist
//	@Security    BearerAuth
//	@Produce     json
//	@Param       movie_id path string true "Movie UUID"
//	@Success     200 {object} map[string]interface{} "entry object or null"
//	@Failure     401 {object} errorResponse
//	@Router      /me/watchlist/{movie_id} [get]
func (h *WatchlistHandler) swaggerGetEntry() {}

// ListEntries godoc
//
//	@Summary     Get watchlist
//	@Description Returns the user's full watchlist, optionally filtered by status.
//	@Tags        Watchlist
//	@Security    BearerAuth
//	@Produce     json
//	@Param       status query string false "Filter: to_watch, watching, watched, dropped"
//	@Param       limit  query int    false "Page size" default(20)
//	@Param       offset query int    false "Pagination offset" default(0)
//	@Success     200 {object} watchlistResponse
//	@Failure     401 {object} errorResponse
//	@Router      /me/watchlist [get]
func (h *WatchlistHandler) swaggerListEntries() {}

// WatchlistStats godoc
//
//	@Summary     Watchlist stats
//	@Description Returns the count of entries per status — used for profile stats
//	             (movies watched, completion rate, etc.).
//	@Tags        Watchlist
//	@Security    BearerAuth
//	@Produce     json
//	@Success     200 {object} map[string]int "status → count map"
//	@Failure     401 {object} errorResponse
//	@Router      /me/watchlist/stats [get]
func (h *WatchlistHandler) swaggerWatchlistStats() {}

// ─── Lists ────────────────────────────────────────────────────────────────────

// GetList godoc
//
//	@Summary     Get list
//	@Description Returns a custom list with all its items. Public and collaborative
//	             lists are accessible without auth. Private lists require ownership.
//	@Tags        Lists
//	@Produce     json
//	@Param       id_or_slug path string true "List UUID or share slug"
//	@Success     200 {object} listDetailResponse
//	@Failure     403 {object} errorResponse "Private list"
//	@Failure     404 {object} errorResponse
//	@Router      /lists/{id_or_slug} [get]
func (h *WatchlistHandler) swaggerGetList() {}

// CreateList godoc
//
//	@Summary     Create list
//	@Description Creates a new custom movie list. Visibility: public, private, or collaborative.
//	@Tags        Lists
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       body body createListRequest true "List details"
//	@Success     201 {object} listResponse
//	@Failure     400 {object} errorResponse
//	@Failure     401 {object} errorResponse
//	@Router      /me/lists [post]
func (h *WatchlistHandler) swaggerCreateList() {}

// MyLists godoc
//
//	@Summary     Get my lists
//	@Description Returns all custom lists owned by the authenticated user.
//	@Tags        Lists
//	@Security    BearerAuth
//	@Produce     json
//	@Param       limit  query int false "Page size" default(20)
//	@Param       offset query int false "Pagination offset" default(0)
//	@Success     200 {object} listPageResponse
//	@Failure     401 {object} errorResponse
//	@Router      /me/lists [get]
func (h *WatchlistHandler) swaggerMyLists() {}

// AddItem godoc
//
//	@Summary     Add item to list
//	@Description Adds a movie to a custom list with an optional note.
//	             Idempotent — adding an already-present movie is a no-op.
//	@Tags        Lists
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       id   path string true "List UUID"
//	@Param       body body addItemRequest true "Movie to add"
//	@Success     201 {object} itemResponse
//	@Failure     400 {object} errorResponse
//	@Failure     401 {object} errorResponse
//	@Failure     403 {object} errorResponse
//	@Router      /me/lists/{id}/items [post]
func (h *WatchlistHandler) swaggerAddItem() {}

// ReorderItems godoc
//
//	@Summary     Reorder list items
//	@Description Sets the sort order for all items by providing the full ordered
//	             array of movie IDs. The request must include every movie currently
//	             in the list.
//	@Tags        Lists
//	@Security    BearerAuth
//	@Accept      json
//	@Param       id   path string true "List UUID"
//	@Param       body body reorderRequest true "Full ordered movie ID array"
//	@Success     204 "Order updated"
//	@Failure     400 {object} errorResponse
//	@Failure     401 {object} errorResponse
//	@Failure     403 {object} errorResponse
//	@Router      /me/lists/{id}/items/order [put]
func (h *WatchlistHandler) swaggerReorderItems() {}

// ─── Schema stubs ─────────────────────────────────────────────────────────────

type watchlistResponse struct {
	Entries []entryResponse `json:"entries"`
	Total   int             `json:"total"`
	Limit   int             `json:"limit"`
	Offset  int             `json:"offset"`
}

type listPageResponse struct {
	Lists  []listResponse `json:"lists"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

type errorResponse struct {
	Error struct {
		Code    string      `json:"code"`
		Message string      `json:"message"`
		Details interface{} `json:"details,omitempty"`
	} `json:"error"`
}
