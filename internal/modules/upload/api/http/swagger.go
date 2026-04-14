package httpapi

// ─── Uploads ─────────────────────────────────────────────────────────────────

// Initiate godoc
//
//	@Summary     Initiate upload
//	@Description Performs quota checks, creates an upload session, and returns a presigned URL.
//	@Tags        Uploads
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       body body initiateRequest true "Upload initiation payload"
//	@Success     201 {object} initiateResponse
//	@Failure     400 {object} errorResponse
//	@Failure     403 {object} errorResponse
//	@Router      /uploads/initiate [post]
func (h *UploadHandler) swaggerInitiate() {}

// GetProgress godoc
//
//	@Summary     Get upload progress
//	@Description Returns the current status and metadata of an upload session.
//	@Tags        Uploads
//	@Security    BearerAuth
//	@Produce     json
//	@Param       id path string true "Upload ID"
//	@Success     200 {object} progressResponse
//	@Failure     404 {object} errorResponse
//	@Router      /uploads/{id} [get]
func (h *UploadHandler) swaggerGetProgress() {}

// Complete godoc
//
//	@Summary     Complete upload
//	@Description Confirms upload completion and triggers processing pipeline.
//	@Tags        Uploads
//	@Security    BearerAuth
//	@Produce     json
//	@Param       id path string true "Upload ID"
//	@Success     200 {object} completeResponse
//	@Failure     400 {object} errorResponse
//	@Failure     404 {object} errorResponse
//	@Router      /uploads/{id}/complete [post]
func (h *UploadHandler) swaggerComplete() {}

// Abort godoc
//
//	@Summary     Abort upload
//	@Description Cancels an upload session and cleans up resources.
//	@Tags        Uploads
//	@Security    BearerAuth
//	@Accept      json
//	@Param       id   path string        true  "Upload ID"
//	@Param       body body abortRequest false "Optional abort reason"
//	@Success     204 "Aborted"
//	@Failure     403 {object} errorResponse
//	@Failure     404 {object} errorResponse
//	@Router      /uploads/{id} [delete]
func (h *UploadHandler) swaggerAbort() {}

// ─── Assets ──────────────────────────────────────────────────────────────────

// ListMyAssets godoc
//
//	@Summary     List my assets
//	@Description Returns paginated list of media assets belonging to the user.
//	@Tags        Uploads
//	@Security    BearerAuth
//	@Produce     json
//	@Param       limit  query int false "Page size" default(20)
//	@Param       offset query int false "Pagination offset" default(0)
//	@Success     200 {object} assetListResponse
//	@Failure     403 {object} errorResponse
//	@Router      /me/assets [get]
func (h *UploadHandler) swaggerListMyAssets() {}

// GetAsset godoc
//
//	@Summary     Get asset
//	@Description Returns a single media asset by ID.
//	@Tags        Uploads
//	@Security    BearerAuth
//	@Produce     json
//	@Param       asset_id path string true "Asset ID"
//	@Success     200 {object} assetResponse
//	@Failure     404 {object} errorResponse
//	@Router      /me/assets/{asset_id} [get]
func (h *UploadHandler) swaggerGetAsset() {}

// ─── Response schema stubs ────────────────────────────────────────────────────

type assetListResponse struct {
	Assets []assetResponse `json:"assets"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

type errorResponse struct {
	Error struct {
		Code    string      `json:"code"`
		Message string      `json:"message"`
		Details interface{} `json:"details,omitempty"`
	} `json:"error"`
}
