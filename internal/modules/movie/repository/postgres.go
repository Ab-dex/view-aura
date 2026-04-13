package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ab-dex/view-aura/internal/modules/movie/domain"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

const pgUniqueViolation = "23505"

// ─── Error mapping ────────────────────────────────────────────────────────────

func mapPgError(err error, op string) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return apierror.Conflict(fmt.Sprintf("%s: duplicate record", op)).WithCause(err)
	}
	return apierror.DatabaseError(fmt.Errorf("%s: %w", op, err))
}

type rowScanner interface{ Scan(dest ...any) error }

// ─── Movie repository ─────────────────────────────────────────────────────────

type pgMovieRepository struct{ pool *pgxpool.Pool }

func NewMovieRepository(pool *pgxpool.Pool) MovieRepository {
	return &pgMovieRepository{pool: pool}
}

var _ MovieRepository = (*pgMovieRepository)(nil)

func (r *pgMovieRepository) Create(ctx context.Context, m *domain.Movie) (*domain.Movie, error) {
	const q = `
		INSERT INTO movies (
			id, title, original_title, slug, synopsis, tagline,
			release_date, runtime_mins, content_rating, status,
			original_lang, countries, genres,
			poster_url, backdrop_url, trailer_url,
			imdb_id, tmdb_id,
			avg_rating, rating_count, popularity_score,
			created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,
			$7,$8,$9,$10,
			$11,$12,$13,
			$14,$15,$16,
			$17,$18,
			0, 0, 0,
			NOW(), NOW()
		)
		RETURNING id, title, original_title, slug, synopsis, tagline,
		          release_date, runtime_mins, content_rating, status,
		          original_lang, countries, genres,
		          poster_url, backdrop_url, trailer_url,
		          imdb_id, tmdb_id,
		          avg_rating, rating_count, popularity_score,
		          created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		m.ID, m.Title, m.OriginalTitle, m.Slug, m.Synopsis, m.Tagline,
		m.ReleaseDate, m.RuntimeMins, m.ContentRating, m.Status,
		m.OriginalLang, m.Countries, m.Genres,
		m.PosterURL, m.BackdropURL, m.TrailerURL,
		m.IMDbID, m.TMDbID,
	)
	result, err := scanMovie(row)
	if err != nil {
		return nil, mapPgError(err, "create movie")
	}
	return result, nil
}

func (r *pgMovieRepository) GetByID(ctx context.Context, id domain.MovieID) (*domain.Movie, error) {
	const q = `
		SELECT id, title, original_title, slug, synopsis, tagline,
		       release_date, runtime_mins, content_rating, status,
		       original_lang, countries, genres,
		       poster_url, backdrop_url, trailer_url,
		       imdb_id, tmdb_id,
		       avg_rating, rating_count, popularity_score,
		       created_at, updated_at
		FROM movies WHERE id = $1 AND status != 'archived'`

	row := r.pool.QueryRow(ctx, q, id)
	m, err := scanMovie(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "movie not found")
		}
		return nil, mapPgError(err, "get movie by id")
	}
	return m, nil
}

func (r *pgMovieRepository) GetBySlug(ctx context.Context, slug string) (*domain.Movie, error) {
	const q = `
		SELECT id, title, original_title, slug, synopsis, tagline,
		       release_date, runtime_mins, content_rating, status,
		       original_lang, countries, genres,
		       poster_url, backdrop_url, trailer_url,
		       imdb_id, tmdb_id,
		       avg_rating, rating_count, popularity_score,
		       created_at, updated_at
		FROM movies WHERE slug = $1 AND status != 'archived'`

	row := r.pool.QueryRow(ctx, q, slug)
	m, err := scanMovie(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "movie not found")
		}
		return nil, mapPgError(err, "get movie by slug")
	}
	return m, nil
}

func (r *pgMovieRepository) Update(ctx context.Context, m *domain.Movie) (*domain.Movie, error) {
	const q = `
		UPDATE movies SET
			title = $2, original_title = $3, synopsis = $4, tagline = $5,
			release_date = $6, runtime_mins = $7, content_rating = $8, status = $9,
			original_lang = $10, countries = $11, genres = $12,
			poster_url = $13, backdrop_url = $14, trailer_url = $15,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, title, original_title, slug, synopsis, tagline,
		          release_date, runtime_mins, content_rating, status,
		          original_lang, countries, genres,
		          poster_url, backdrop_url, trailer_url,
		          imdb_id, tmdb_id,
		          avg_rating, rating_count, popularity_score,
		          created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		m.ID, m.Title, m.OriginalTitle, m.Synopsis, m.Tagline,
		m.ReleaseDate, m.RuntimeMins, m.ContentRating, m.Status,
		m.OriginalLang, m.Countries, m.Genres,
		m.PosterURL, m.BackdropURL, m.TrailerURL,
	)
	result, err := scanMovie(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "movie not found")
		}
		return nil, mapPgError(err, "update movie")
	}
	return result, nil
}

func (r *pgMovieRepository) Delete(ctx context.Context, id domain.MovieID) error {
	const q = `UPDATE movies SET status = 'archived', updated_at = NOW() WHERE id = $1`
	ct, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return mapPgError(err, "archive movie")
	}
	if ct.RowsAffected() == 0 {
		return apierror.New(404, apierror.CodeNotFound, "movie not found")
	}
	return nil
}

func (r *pgMovieRepository) List(ctx context.Context, f domain.MovieFilter) ([]*domain.Movie, int, error) {
	// Build the WHERE clause dynamically from the filter.
	var conds []string
	var args []any
	i := 1

	arg := func(v any) string {
		args = append(args, v)
		s := fmt.Sprintf("$%d", i)
		i++
		return s
	}

	conds = append(conds, "status = 'published'")

	if f.Query != "" {
		conds = append(conds, fmt.Sprintf(
			"(to_tsvector('english', title || ' ' || synopsis) @@ plainto_tsquery('english', %s))",
			arg(f.Query),
		))
	}
	if len(f.Genres) > 0 {
		conds = append(conds, fmt.Sprintf("genres && %s", arg(f.Genres)))
	}
	if len(f.ContentRatings) > 0 {
		conds = append(conds, fmt.Sprintf("content_rating = ANY(%s)", arg(f.ContentRatings)))
	}
	if f.YearFrom != nil {
		conds = append(conds, fmt.Sprintf("EXTRACT(YEAR FROM release_date) >= %s", arg(*f.YearFrom)))
	}
	if f.YearTo != nil {
		conds = append(conds, fmt.Sprintf("EXTRACT(YEAR FROM release_date) <= %s", arg(*f.YearTo)))
	}
	if f.RuntimeMax != nil {
		conds = append(conds, fmt.Sprintf("runtime_mins <= %s", arg(*f.RuntimeMax)))
	}
	if f.MinRating != nil {
		conds = append(conds, fmt.Sprintf("avg_rating >= %s", arg(*f.MinRating)))
	}

	where := "WHERE " + strings.Join(conds, " AND ")

	sortCol := "popularity_score"
	switch f.SortBy {
	case "release_date":
		sortCol = "release_date"
	case "avg_rating":
		sortCol = "avg_rating"
	case "title":
		sortCol = "title"
	}
	dir := "DESC"
	if strings.ToUpper(f.SortDir) == "ASC" {
		dir = "ASC"
	}

	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	countQ := fmt.Sprintf("SELECT COUNT(*) FROM movies %s", where)
	var total int
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, mapPgError(err, "count movies")
	}

	listQ := fmt.Sprintf(`
		SELECT id, title, original_title, slug, synopsis, tagline,
		       release_date, runtime_mins, content_rating, status,
		       original_lang, countries, genres,
		       poster_url, backdrop_url, trailer_url,
		       imdb_id, tmdb_id,
		       avg_rating, rating_count, popularity_score,
		       created_at, updated_at
		FROM movies %s
		ORDER BY %s %s
		LIMIT %s OFFSET %s`,
		where, sortCol, dir, arg(limit), arg(offset),
	)

	rows, err := r.pool.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, mapPgError(err, "list movies")
	}
	defer rows.Close()

	var movies []*domain.Movie
	for rows.Next() {
		m, err := scanMovie(rows)
		if err != nil {
			return nil, 0, mapPgError(err, "scan movie row")
		}
		movies = append(movies, m)
	}
	return movies, total, rows.Err()
}

func (r *pgMovieRepository) UpdateAggregates(ctx context.Context, id domain.MovieID, avgRating float64, ratingCount int, popularity float64) error {
	const q = `
		UPDATE movies
		SET avg_rating = $2, rating_count = $3, popularity_score = $4, updated_at = NOW()
		WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id, avgRating, ratingCount, popularity)
	return mapPgError(err, "update movie aggregates")
}

func (r *pgMovieRepository) ExistsByIMDbID(ctx context.Context, imdbID string) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM movies WHERE imdb_id = $1)`
	var exists bool
	if err := r.pool.QueryRow(ctx, q, imdbID).Scan(&exists); err != nil {
		return false, mapPgError(err, "exists by imdb id")
	}
	return exists, nil
}

func scanMovie(row rowScanner) (*domain.Movie, error) {
	var m domain.Movie
	err := row.Scan(
		&m.ID, &m.Title, &m.OriginalTitle, &m.Slug, &m.Synopsis, &m.Tagline,
		&m.ReleaseDate, &m.RuntimeMins, &m.ContentRating, &m.Status,
		&m.OriginalLang, &m.Countries, &m.Genres,
		&m.PosterURL, &m.BackdropURL, &m.TrailerURL,
		&m.IMDbID, &m.TMDbID,
		&m.AvgRating, &m.RatingCount, &m.PopularityScore,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ─── Genre repository ─────────────────────────────────────────────────────────

type pgGenreRepository struct{ pool *pgxpool.Pool }

func NewGenreRepository(pool *pgxpool.Pool) GenreRepository {
	return &pgGenreRepository{pool: pool}
}

var _ GenreRepository = (*pgGenreRepository)(nil)

func (r *pgGenreRepository) Create(ctx context.Context, g *domain.Genre) (*domain.Genre, error) {
	const q = `
		INSERT INTO genres (id, name, slug, description)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, slug, description`
	row := r.pool.QueryRow(ctx, q, g.ID, g.Name, g.Slug, g.Description)
	var out domain.Genre
	if err := row.Scan(&out.ID, &out.Name, &out.Slug, &out.Description); err != nil {
		return nil, mapPgError(err, "create genre")
	}
	return &out, nil
}

func (r *pgGenreRepository) GetByID(ctx context.Context, id domain.GenreID) (*domain.Genre, error) {
	const q = `SELECT id, name, slug, description FROM genres WHERE id = $1`
	var g domain.Genre
	if err := r.pool.QueryRow(ctx, q, id).Scan(&g.ID, &g.Name, &g.Slug, &g.Description); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "genre not found")
		}
		return nil, mapPgError(err, "get genre by id")
	}
	return &g, nil
}

func (r *pgGenreRepository) GetBySlug(ctx context.Context, slug string) (*domain.Genre, error) {
	const q = `SELECT id, name, slug, description FROM genres WHERE slug = $1`
	var g domain.Genre
	if err := r.pool.QueryRow(ctx, q, slug).Scan(&g.ID, &g.Name, &g.Slug, &g.Description); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "genre not found")
		}
		return nil, mapPgError(err, "get genre by slug")
	}
	return &g, nil
}

func (r *pgGenreRepository) List(ctx context.Context) ([]*domain.Genre, error) {
	const q = `SELECT id, name, slug, description FROM genres ORDER BY name ASC`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, mapPgError(err, "list genres")
	}
	defer rows.Close()

	var out []*domain.Genre
	for rows.Next() {
		var g domain.Genre
		if err := rows.Scan(&g.ID, &g.Name, &g.Slug, &g.Description); err != nil {
			return nil, mapPgError(err, "scan genre")
		}
		out = append(out, &g)
	}
	return out, rows.Err()
}

func (r *pgGenreRepository) Delete(ctx context.Context, id domain.GenreID) error {
	const q = `DELETE FROM genres WHERE id = $1`
	ct, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return mapPgError(err, "delete genre")
	}
	if ct.RowsAffected() == 0 {
		return apierror.New(404, apierror.CodeNotFound, "genre not found")
	}
	return nil
}

// ─── Person repository ────────────────────────────────────────────────────────

type pgPersonRepository struct{ pool *pgxpool.Pool }

func NewPersonRepository(pool *pgxpool.Pool) PersonRepository {
	return &pgPersonRepository{pool: pool}
}

var _ PersonRepository = (*pgPersonRepository)(nil)

func (r *pgPersonRepository) Create(ctx context.Context, p *domain.Person) (*domain.Person, error) {
	const q = `
		INSERT INTO persons (id, name, slug, birth_date, death_date, birthplace, bio, profile_url, imdb_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW(),NOW())
		RETURNING id, name, slug, birth_date, death_date, birthplace, bio, profile_url, imdb_id, created_at, updated_at`
	row := r.pool.QueryRow(ctx, q, p.ID, p.Name, p.Slug, p.BirthDate, p.DeathDate, p.Birthplace, p.Bio, p.ProfileURL, p.IMDbID)
	return scanPerson(row)
}

func (r *pgPersonRepository) GetByID(ctx context.Context, id domain.PersonID) (*domain.Person, error) {
	const q = `SELECT id, name, slug, birth_date, death_date, birthplace, bio, profile_url, imdb_id, created_at, updated_at FROM persons WHERE id = $1`
	p, err := scanPerson(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "person not found")
		}
		return nil, mapPgError(err, "get person by id")
	}
	return p, nil
}

func (r *pgPersonRepository) GetBySlug(ctx context.Context, slug string) (*domain.Person, error) {
	const q = `SELECT id, name, slug, birth_date, death_date, birthplace, bio, profile_url, imdb_id, created_at, updated_at FROM persons WHERE slug = $1`
	p, err := scanPerson(r.pool.QueryRow(ctx, q, slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "person not found")
		}
		return nil, mapPgError(err, "get person by slug")
	}
	return p, nil
}

func (r *pgPersonRepository) Search(ctx context.Context, query string, limit int) ([]*domain.Person, error) {
	const q = `
		SELECT id, name, slug, birth_date, death_date, birthplace, bio, profile_url, imdb_id, created_at, updated_at
		FROM persons
		WHERE name ILIKE '%' || $1 || '%'
		ORDER BY name ASC LIMIT $2`
	rows, err := r.pool.Query(ctx, q, query, limit)
	if err != nil {
		return nil, mapPgError(err, "search persons")
	}
	defer rows.Close()

	var out []*domain.Person
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, mapPgError(err, "scan person")
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *pgPersonRepository) Update(ctx context.Context, p *domain.Person) (*domain.Person, error) {
	const q = `
		UPDATE persons SET name=$2, birth_date=$3, death_date=$4, birthplace=$5, bio=$6, profile_url=$7, updated_at=NOW()
		WHERE id=$1
		RETURNING id, name, slug, birth_date, death_date, birthplace, bio, profile_url, imdb_id, created_at, updated_at`
	result, err := scanPerson(r.pool.QueryRow(ctx, q, p.ID, p.Name, p.BirthDate, p.DeathDate, p.Birthplace, p.Bio, p.ProfileURL))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "person not found")
		}
		return nil, mapPgError(err, "update person")
	}
	return result, nil
}

func scanPerson(row rowScanner) (*domain.Person, error) {
	var p domain.Person
	err := row.Scan(&p.ID, &p.Name, &p.Slug, &p.BirthDate, &p.DeathDate, &p.Birthplace, &p.Bio, &p.ProfileURL, &p.IMDbID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ─── Credit repository ────────────────────────────────────────────────────────

type pgCreditRepository struct{ pool *pgxpool.Pool }

func NewCreditRepository(pool *pgxpool.Pool) CreditRepository {
	return &pgCreditRepository{pool: pool}
}

var _ CreditRepository = (*pgCreditRepository)(nil)

func (r *pgCreditRepository) Add(ctx context.Context, c *domain.Credit) (*domain.Credit, error) {
	const q = `
		INSERT INTO movie_credits (id, movie_id, person_id, role, character, billing_order)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, movie_id, person_id, role, character, billing_order`
	var out domain.Credit
	err := r.pool.QueryRow(ctx, q, c.ID, c.MovieID, c.PersonID, c.Role, c.Character, c.BillingOrder).
		Scan(&out.ID, &out.MovieID, &out.PersonID, &out.Role, &out.Character, &out.BillingOrder)
	if err != nil {
		return nil, mapPgError(err, "add credit")
	}
	return &out, nil
}

func (r *pgCreditRepository) Remove(ctx context.Context, creditID string) error {
	const q = `DELETE FROM movie_credits WHERE id = $1`
	ct, err := r.pool.Exec(ctx, q, creditID)
	if err != nil {
		return mapPgError(err, "remove credit")
	}
	if ct.RowsAffected() == 0 {
		return apierror.New(404, apierror.CodeNotFound, "credit not found")
	}
	return nil
}

func (r *pgCreditRepository) ListByMovie(ctx context.Context, movieID domain.MovieID) ([]*domain.Credit, error) {
	const q = `
		SELECT id, movie_id, person_id, role, character, billing_order
		FROM movie_credits WHERE movie_id = $1
		ORDER BY billing_order ASC, role ASC`
	return r.queryCredits(ctx, q, movieID)
}

func (r *pgCreditRepository) ListByPerson(ctx context.Context, personID domain.PersonID) ([]*domain.Credit, error) {
	const q = `
		SELECT id, movie_id, person_id, role, character, billing_order
		FROM movie_credits WHERE person_id = $1
		ORDER BY role ASC`
	return r.queryCredits(ctx, q, personID)
}

func (r *pgCreditRepository) queryCredits(ctx context.Context, q string, arg any) ([]*domain.Credit, error) {
	rows, err := r.pool.Query(ctx, q, arg)
	if err != nil {
		return nil, mapPgError(err, "query credits")
	}
	defer rows.Close()

	var out []*domain.Credit
	for rows.Next() {
		var c domain.Credit
		if err := rows.Scan(&c.ID, &c.MovieID, &c.PersonID, &c.Role, &c.Character, &c.BillingOrder); err != nil {
			return nil, mapPgError(err, "scan credit")
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

// ─── Streaming link repository ────────────────────────────────────────────────

type pgStreamingLinkRepository struct{ pool *pgxpool.Pool }

func NewStreamingLinkRepository(pool *pgxpool.Pool) StreamingLinkRepository {
	return &pgStreamingLinkRepository{pool: pool}
}

var _ StreamingLinkRepository = (*pgStreamingLinkRepository)(nil)

func (r *pgStreamingLinkRepository) Upsert(ctx context.Context, l *domain.StreamingLink) (*domain.StreamingLink, error) {
	const q = `
		INSERT INTO streaming_links (id, movie_id, provider, link_url, access_type, price_cents, region, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())
		ON CONFLICT (movie_id, provider, region) DO UPDATE SET
			link_url    = EXCLUDED.link_url,
			access_type = EXCLUDED.access_type,
			price_cents = EXCLUDED.price_cents,
			updated_at  = NOW()
		RETURNING id, movie_id, provider, link_url, access_type, price_cents, region, updated_at`

	var out domain.StreamingLink
	err := r.pool.QueryRow(ctx, q, l.ID, l.MovieID, l.Provider, l.LinkURL, l.AccessType, l.PriceCents, l.Region).
		Scan(&out.ID, &out.MovieID, &out.Provider, &out.LinkURL, &out.AccessType, &out.PriceCents, &out.Region, &out.UpdatedAt)
	if err != nil {
		return nil, mapPgError(err, "upsert streaming link")
	}
	return &out, nil
}

func (r *pgStreamingLinkRepository) ListByMovie(ctx context.Context, movieID domain.MovieID, region string) ([]*domain.StreamingLink, error) {
	const q = `
		SELECT id, movie_id, provider, link_url, access_type, price_cents, region, updated_at
		FROM streaming_links
		WHERE movie_id = $1 AND (region = $2 OR region = '')
		ORDER BY provider ASC`

	rows, err := r.pool.Query(ctx, q, movieID, region)
	if err != nil {
		return nil, mapPgError(err, "list streaming links")
	}
	defer rows.Close()

	var out []*domain.StreamingLink
	for rows.Next() {
		var l domain.StreamingLink
		if err := rows.Scan(&l.ID, &l.MovieID, &l.Provider, &l.LinkURL, &l.AccessType, &l.PriceCents, &l.Region, &l.UpdatedAt); err != nil {
			return nil, mapPgError(err, "scan streaming link")
		}
		out = append(out, &l)
	}
	return out, rows.Err()
}

func (r *pgStreamingLinkRepository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM streaming_links WHERE id = $1`
	ct, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return mapPgError(err, "delete streaming link")
	}
	if ct.RowsAffected() == 0 {
		return apierror.New(404, apierror.CodeNotFound, "streaming link not found")
	}
	return nil
}

// ─── Filming location repository ──────────────────────────────────────────────

type pgFilmingLocationRepository struct{ pool *pgxpool.Pool }

func NewFilmingLocationRepository(pool *pgxpool.Pool) FilmingLocationRepository {
	return &pgFilmingLocationRepository{pool: pool}
}

var _ FilmingLocationRepository = (*pgFilmingLocationRepository)(nil)

func (r *pgFilmingLocationRepository) Add(ctx context.Context, l *domain.FilmingLocation) (*domain.FilmingLocation, error) {
	const q = `
		INSERT INTO filming_locations (id, movie_id, name, latitude, longitude, description)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, movie_id, name, latitude, longitude, description`
	var out domain.FilmingLocation
	err := r.pool.QueryRow(ctx, q, l.ID, l.MovieID, l.Name, l.Latitude, l.Longitude, l.Description).
		Scan(&out.ID, &out.MovieID, &out.Name, &out.Latitude, &out.Longitude, &out.Description)
	if err != nil {
		return nil, mapPgError(err, "add filming location")
	}
	return &out, nil
}

func (r *pgFilmingLocationRepository) ListByMovie(ctx context.Context, movieID domain.MovieID) ([]*domain.FilmingLocation, error) {
	const q = `SELECT id, movie_id, name, latitude, longitude, description FROM filming_locations WHERE movie_id = $1`
	rows, err := r.pool.Query(ctx, q, movieID)
	if err != nil {
		return nil, mapPgError(err, "list filming locations")
	}
	defer rows.Close()

	var out []*domain.FilmingLocation
	for rows.Next() {
		var l domain.FilmingLocation
		if err := rows.Scan(&l.ID, &l.MovieID, &l.Name, &l.Latitude, &l.Longitude, &l.Description); err != nil {
			return nil, mapPgError(err, "scan filming location")
		}
		out = append(out, &l)
	}
	return out, rows.Err()
}

func (r *pgFilmingLocationRepository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM filming_locations WHERE id = $1`
	ct, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return mapPgError(err, "delete filming location")
	}
	if ct.RowsAffected() == 0 {
		return apierror.New(404, apierror.CodeNotFound, "filming location not found")
	}
	return nil
}

// Suppress unused import.
var _ = time.Now
