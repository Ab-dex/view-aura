package httpapi

// ListHouseholdProfiles godoc
//
//	@Summary     List household profiles
//	@Description Returns all viewing profiles for the authenticated user's account,
//	             ordered by sort_order. Every account has at least one profile.
//	@Tags        Profiles
//	@Security    BearerAuth
//	@Produce     json
//	@Success     200 {object} map[string][]householdProfileResponse "profiles array"
//	@Failure     401 {object} errorResponse
//	@Router      /users/me/profiles [get]
func (h *ProfileHandler) swaggerList() {}

// CreateHouseholdProfile godoc
//
//	@Summary     Create a household profile
//	@Description Adds a new viewing profile to the account (max 5 per account).
//	             The first profile created auto-becomes the default.
//	             Kids profiles are automatically capped at PG regardless of max_rating.
//	@Tags        Profiles
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       body body createHouseholdProfileRequest true "Profile to create"
//	@Success     201 {object} householdProfileResponse
//	@Failure     400 {object} errorResponse "Validation error"
//	@Failure     401 {object} errorResponse
//	@Failure     409 {object} errorResponse "Profile limit reached"
//	@Router      /users/me/profiles [post]
func (h *ProfileHandler) swaggerCreate() {}

// GetHouseholdProfile godoc
//
//	@Summary     Get a household profile
//	@Description Returns a single profile by ID. The profile must belong to
//	             the authenticated user.
//	@Tags        Profiles
//	@Security    BearerAuth
//	@Produce     json
//	@Param       id path string true "Profile ID"
//	@Success     200 {object} householdProfileResponse
//	@Failure     401 {object} errorResponse
//	@Failure     403 {object} errorResponse "Profile belongs to another account"
//	@Failure     404 {object} errorResponse
//	@Router      /users/me/profiles/{id} [get]
func (h *ProfileHandler) swaggerGet() {}

// UpdateHouseholdProfile godoc
//
//	@Summary     Update a household profile
//	@Description Applies partial updates. All fields are optional — only provided
//	             fields are changed. Send an empty string for pin to remove it.
//	@Tags        Profiles
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       id   path string                         true "Profile ID"
//	@Param       body body updateHouseholdProfileRequest  true "Fields to update"
//	@Success     200 {object} householdProfileResponse
//	@Failure     400 {object} errorResponse
//	@Failure     401 {object} errorResponse
//	@Failure     403 {object} errorResponse
//	@Failure     404 {object} errorResponse
//	@Router      /users/me/profiles/{id} [patch]
func (h *ProfileHandler) swaggerUpdate() {}

// DeleteHouseholdProfile godoc
//
//	@Summary     Delete a household profile
//	@Description Removes a profile. Cannot delete the last profile on an account
//	             or the current default profile (set another as default first).
//	@Tags        Profiles
//	@Security    BearerAuth
//	@Param       id path string true "Profile ID"
//	@Success     204 "Profile deleted"
//	@Failure     401 {object} errorResponse
//	@Failure     403 {object} errorResponse
//	@Failure     404 {object} errorResponse
//	@Failure     409 {object} errorResponse "Last profile or default profile"
//	@Router      /users/me/profiles/{id} [delete]
func (h *ProfileHandler) swaggerDelete() {}

// SwitchHouseholdProfile godoc
//
//	@Summary     Switch active profile
//	@Description Validates the PIN (when required) and returns the profile record.
//	             The client should store the returned profile_id and include it
//	             in subsequent requests that are profile-scoped.
//	@Tags        Profiles
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       id   path string                        true "Profile ID"
//	@Param       body body switchHouseholdProfileRequest false "PIN (required only when profile has one)"
//	@Success     200 {object} map[string]interface{} "active_profile + message"
//	@Failure     401 {object} errorResponse
//	@Failure     403 {object} errorResponse "Wrong PIN or not your profile"
//	@Failure     404 {object} errorResponse
//	@Router      /users/me/profiles/{id}/switch [post]
func (h *ProfileHandler) swaggerSwitch() {}

// SetDefaultHouseholdProfile godoc
//
//	@Summary     Set default profile
//	@Description Marks a profile as the account default. The previous default is
//	             cleared atomically in a single transaction.
//	@Tags        Profiles
//	@Security    BearerAuth
//	@Param       id path string true "Profile ID"
//	@Success     200 {object} map[string]string "message"
//	@Failure     401 {object} errorResponse
//	@Failure     403 {object} errorResponse
//	@Failure     404 {object} errorResponse
//	@Router      /users/me/profiles/{id}/default [put]
func (h *ProfileHandler) swaggerSetDefault() {}

type householdProfileResponse struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Type      string `json:"type"`
	IsDefault bool   `json:"is_default"`
	SortOrder int    `json:"sort_order"`
	CreatedAt string `json:"created_at"`
}

type createHouseholdProfileRequest struct {
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Type      string `json:"type"`
	PIN       string `json:"pin"`
	MaxRating string `json:"max_rating"`
}

type updateHouseholdProfileRequest struct {
	Name      *string `json:"name"`
	AvatarURL *string `json:"avatar_url"`
	PIN       *string `json:"pin"`
	MaxRating *string `json:"max_rating"`
	IsDefault *bool   `json:"is_default"`
	SortOrder *int    `json:"sort_order"`
}

type switchHouseholdProfileRequest struct {
	PIN string `json:"pin"`
}

type errorResponse struct {
	Error struct {
		Code    string      `json:"code"`
		Message string      `json:"message"`
		Details interface{} `json:"details,omitempty"`
	} `json:"error"`
}
