package domain

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func ParseMovieFilter(c *gin.Context) (MovieFilter, error) {
	var f MovieFilter

	// ─── basic query ───────────────────────
	f.Query = strings.TrimSpace(c.Query("q"))

	f.SortBy = c.DefaultQuery("sort_by", "popularity_score")
	f.SortDir = c.DefaultQuery("sort_dir", "desc")

	if err := validateSort(&f); err != nil {
		return f, err
	}

	// ─── genres ────────────────────────────
	f.Genres = c.QueryArray("genre")
	if f.Genres == nil {
		f.Genres = []string{}
	}

	// ─── content ratings ───────────────────
	if err := parseContentRatings(c, &f); err != nil {
		return f, err
	}

	// ─── year range ────────────────────────
	if err := parseYearRange(c, &f); err != nil {
		return f, err
	}

	// ─── runtime ───────────────────────────
	if err := parseRuntime(c, &f); err != nil {
		return f, err
	}

	// ─── rating filter ─────────────────────
	if err := parseMinRating(c, &f); err != nil {
		return f, err
	}

	// ─── pagination ────────────────────────
	f.Limit = clampInt(c.DefaultQuery("limit", "20"), 1, 100)
	f.Offset = clampInt(c.DefaultQuery("offset", "0"), 0, 100000)

	return f, nil
}

func parseContentRatings(c *gin.Context, f *MovieFilter) error {
	raw := c.QueryArray("content_rating")
	if len(raw) == 0 {
		f.ContentRatings = nil
		return nil
	}

	seen := map[ContentRating]struct{}{}
	f.ContentRatings = make([]ContentRating, 0, len(raw))

	for _, r := range raw {
		cr := ContentRating(strings.TrimSpace(r))

		if !cr.IsValid() {
			return fmt.Errorf("invalid content_rating: %s", r)
		}

		if _, ok := seen[cr]; ok {
			continue
		}
		seen[cr] = struct{}{}

		f.ContentRatings = append(f.ContentRatings, cr)
	}

	return nil
}

func parseYearRange(c *gin.Context, f *MovieFilter) error {
	if v := c.Query("year_from"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1800 {
			return fmt.Errorf("invalid year_from")
		}
		f.YearFrom = &n
	}

	if v := c.Query("year_to"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid year_to")
		}
		f.YearTo = &n
	}

	if f.YearFrom != nil && f.YearTo != nil && *f.YearFrom > *f.YearTo {
		return fmt.Errorf("year_from cannot be greater than year_to")
	}

	return nil
}

func parseRuntime(c *gin.Context, f *MovieFilter) error {
	if v := c.Query("runtime_max"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return fmt.Errorf("invalid runtime_max")
		}
		f.RuntimeMax = &n
	}
	return nil
}

func parseMinRating(c *gin.Context, f *MovieFilter) error {
	if v := c.Query("min_rating"); v != "" {
		val, err := strconv.ParseFloat(v, 64)
		if err != nil || val < 0 || val > 10 {
			return fmt.Errorf("invalid min_rating")
		}
		f.MinRating = &val
	}
	return nil
}

func validateSort(f *MovieFilter) error {
	switch f.SortBy {
	case "popularity_score", "release_date", "avg_rating", "created_at":
	default:
		return fmt.Errorf("invalid sort_by: %s", f.SortBy)
	}

	switch f.SortDir {
	case "asc", "desc":
	default:
		return fmt.Errorf("invalid sort_dir: %s", f.SortDir)
	}

	return nil
}

func clampInt(v string, min, max int) int {
	n, err := strconv.Atoi(v)
	if err != nil {
		return min
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}
