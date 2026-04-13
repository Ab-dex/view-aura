package httpapi

// List godoc
//
//	@Summary     List notifications
//	@Description Returns the authenticated user's notification inbox, newest-first.
//	             Pass `?unread=true` to fetch only unread notifications.
//	@Tags        Notifications
//	@Security    BearerAuth
//	@Produce     json
//	@Param       unread query bool false "Filter to unread only" default(false)
//	@Param       limit  query int  false "Page size" default(20)
//	@Param       offset query int  false "Pagination offset" default(0)
//	@Success     200 {object} notificationListResponse
//	@Failure     401 {object} errorResponse
//	@Router      /me/notifications [get]
func (h *NotificationHandler) swaggerList() {}

// UnreadCount godoc
//
//	@Summary     Unread badge count
//	@Description Returns the number of unread notifications — used for the badge
//	             counter in the app header and tab bar.
//	@Tags        Notifications
//	@Security    BearerAuth
//	@Produce     json
//	@Success     200 {object} map[string]int "unread_count"
//	@Failure     401 {object} errorResponse
//	@Router      /me/notifications/unread-count [get]
func (h *NotificationHandler) swaggerUnreadCount() {}

// MarkRead godoc
//
//	@Summary     Mark notification read
//	@Description Marks a single notification as read and sets read_at timestamp.
//	             Idempotent if already read.
//	@Tags        Notifications
//	@Security    BearerAuth
//	@Param       id path string true "Notification UUID"
//	@Success     204 "Marked read"
//	@Failure     401 {object} errorResponse
//	@Failure     404 {object} errorResponse
//	@Router      /me/notifications/{id}/read [patch]
func (h *NotificationHandler) swaggerMarkRead() {}

// MarkAllRead godoc
//
//	@Summary     Mark all read
//	@Description Marks every unread notification for the user as read in one call.
//	@Tags        Notifications
//	@Security    BearerAuth
//	@Success     204 "All marked read"
//	@Failure     401 {object} errorResponse
//	@Router      /me/notifications/read-all [patch]
func (h *NotificationHandler) swaggerMarkAllRead() {}

// GetPreferences godoc
//
//	@Summary     Get notification preferences
//	@Description Returns all stored preference rows (type × channel).
//	             Missing rows should be treated as enabled (opt-out model).
//	@Tags        Notifications
//	@Security    BearerAuth
//	@Produce     json
//	@Success     200 {object} preferenceListResponse
//	@Failure     401 {object} errorResponse
//	@Router      /me/notification-preferences [get]
func (h *NotificationHandler) swaggerGetPreferences() {}

// BulkSetPreferences godoc
//
//	@Summary     Bulk update preferences
//	@Description Replaces all notification preferences in one request. The client
//	             should send the full matrix of type × channel settings.
//	@Tags        Notifications
//	@Security    BearerAuth
//	@Accept      json
//	@Param       body body bulkPreferenceRequest true "Full preference matrix"
//	@Success     204 "Preferences updated"
//	@Failure     400 {object} errorResponse
//	@Failure     401 {object} errorResponse
//	@Router      /me/notification-preferences [put]
func (h *NotificationHandler) swaggerBulkSetPreferences() {}

// RegisterPushToken godoc
//
//	@Summary     Register push token
//	@Description Registers a device push token (FCM/APNs) for the authenticated
//	             user. Call on app launch and after notification permission is granted.
//	             Platform must be: ios, android, or web.
//	@Tags        Notifications
//	@Security    BearerAuth
//	@Accept      json
//	@Param       body body pushTokenRequest true "Token and platform"
//	@Success     204 "Token registered"
//	@Failure     400 {object} errorResponse
//	@Failure     401 {object} errorResponse
//	@Router      /me/push-tokens [post]
func (h *NotificationHandler) swaggerRegisterPushToken() {}

// UnregisterPushToken godoc
//
//	@Summary     Unregister push token
//	@Description Removes a push token. Call on logout from a specific device
//	             to stop push delivery to that device.
//	@Tags        Notifications
//	@Security    BearerAuth
//	@Accept      json
//	@Param       body body map[string]string true "Token to remove"
//	@Success     204 "Token removed"
//	@Failure     401 {object} errorResponse
//	@Router      /me/push-tokens [delete]
func (h *NotificationHandler) swaggerUnregisterPushToken() {}

type notificationListResponse struct {
	Notifications []notificationResponse `json:"notifications"`
	Total         int                    `json:"total"`
	Limit         int                    `json:"limit"`
	Offset        int                    `json:"offset"`
}

type preferenceListResponse struct {
	Preferences []preferenceResponse `json:"preferences"`
}

type errorResponse struct {
	Error struct {
		Code    string      `json:"code"`
		Message string      `json:"message"`
		Details interface{} `json:"details,omitempty"`
	} `json:"error"`
}
