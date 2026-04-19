use serde::{Deserialize, Serialize};

/// MovieDocument is the denormalised document stored in the MeiliSearch
/// `movies` index. It is written by the Kafka consumer on every
/// SearchIndexRequested event and updated by MovieRatingAggregateUpdated
/// events (to keep avg_rating and popularity_score fresh).
///
/// The document is intentionally wider than the API response — it includes
/// all fields that the search engine needs for filtering and ranking even
/// if they are not returned to the client.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct MovieDocument {
    /// Primary key — matches movies.id UUID as a String.
    pub id: String,

    pub title:           String,
    pub original_title:  Option<String>,
    pub slug:            String,
    pub synopsis:        Option<String>,
    pub tagline:         Option<String>,

    /// ISO-8601 date string "YYYY-MM-DD" — stored as string for filtering.
    pub release_date:    Option<String>,
    /// Release year extracted from release_date for fast facet queries.
    pub release_year:    Option<i32>,
    pub runtime_mins:    Option<i32>,
    pub content_rating:  Option<String>,
    pub status:          String,
    pub original_lang:   String,

    /// Filterable and facetable arrays.
    pub genres:          Vec<String>,
    pub countries:       Vec<String>,

    pub poster_url:      Option<String>,
    pub backdrop_url:    Option<String>,
    pub trailer_url:     Option<String>,

    pub imdb_id:         Option<String>,
    pub tmdb_id:         Option<i32>,

    // Ranking signals — updated asynchronously.
    pub avg_rating:       f64,
    pub rating_count:     i64,
    /// Weighted recency popularity score from mv_movie_rating_agg.
    pub popularity_score: f64,

    // Denormalised for "Where to Watch" facet filter.
    pub streaming_providers: Vec<String>,

    // Cast / crew names for full-text matching ("movies with DiCaprio").
    pub cast_names:      Vec<String>,
    pub director_names:  Vec<String>,
}

/// PersonDocument is the denormalised document for the `persons` index.
/// Used for cast/crew search ("find all movies with Nolan").
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct PersonDocument {
    pub id:          String,
    pub name:        String,
    pub slug:        String,
    pub bio:         Option<String>,
    pub profile_url: Option<String>,
    pub imdb_id:     Option<String>,
    /// Known role types: "actor", "director", "writer", "composer", etc.
    pub roles:       Vec<String>,
    /// Movie IDs this person is credited on (used to cross-link results).
    pub movie_ids:   Vec<String>,
}