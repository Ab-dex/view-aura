package api

import (
	httpapi "github.com/Ab-dex/view-aura/internal/modules/rating/api/http"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(httpapi.NewRatingHandler)
