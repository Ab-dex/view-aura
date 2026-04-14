// Package proxy implements the reverse proxy to the upstream monolith.
package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/rs/zerolog/log"
)

// New creates a reverse proxy that forwards all requests to upstreamURL.
// The proxy strips the X-Forwarded-For header from untrusted clients and
// re-sets it to the real client IP so the app sees the original address.
func New(upstreamURL string) http.Handler {
	target, err := url.Parse(upstreamURL)
	if err != nil {
		log.Fatal().Err(err).Str("upstream", upstreamURL).
			Msg("proxy: invalid upstream URL")
	}

	rp := httputil.NewSingleHostReverseProxy(target)

	// Customise the director to set the correct Host header and forward
	// the request ID and real IP inserted by upstream middleware.
	defaultDirector := rp.Director
	rp.Director = func(req *http.Request) {
		defaultDirector(req)
		req.Host = target.Host
		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Header.Set("X-Forwarded-Proto", scheme(req))
	}

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Error().Err(err).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("proxy: upstream error")
		http.Error(w,
			fmt.Sprintf(`{"error":{"code":"UPSTREAM_ERROR","message":"%s"}}`, err.Error()),
			http.StatusBadGateway,
		)
	}

	return rp
}

func scheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
