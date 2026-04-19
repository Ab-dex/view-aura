use meilisearch_sdk::settings::Settings;

/// Movie index settings:
/// - Searchable fields ordered by ranking weight (title first).
/// - Filterable attributes for client-side faceted search.
/// - Sortable attributes for "sort by release date / rating".
/// - Ranking rules with our custom `popularity_score` typo-boost.
pub fn movie_index_settings() -> Settings {
    Settings::new()
        .with_searchable_attributes([
            "title",
            "original_title",
            "synopsis",
            "tagline",
            "cast_names",
            "director_names",
            "genres",
        ])
        .with_filterable_attributes([
            "status",
            "genres",
            "countries",
            "content_rating",
            "release_year",
            "original_lang",
            "avg_rating",
            "streaming_providers",
        ])
        .with_sortable_attributes([
            "avg_rating",
            "popularity_score",
            "release_date",
            "rating_count",
        ])
        // Ranking rules: MeiliSearch default + custom popularity booster.
        // Order matters — each rule is a tiebreaker for the previous.
        .with_ranking_rules([
            "words",           // all query words present
            "typo",            // fewer typos wins
            "proximity",       // words closer together wins
            "attribute",       // match in searchable field order (title > synopsis)
            "sort",            // respect client's sort parameter
            "exactness",       // exact match wins over partial
            "popularity_score:desc", // final tiebreaker: popularity
        ])
        .with_typo_tolerance(
            meilisearch_sdk::settings::TypoToleranceSettings::new()
                .with_min_word_size_for_typos(
                    meilisearch_sdk::settings::MinWordSizeForTypos::new()
                        .with_one_typo(4)
                        .with_two_typos(8),
                ),
        )
        .with_distinct_attribute("id")
}

/// Person index settings.
pub fn person_index_settings() -> Settings {
    Settings::new()
        .with_searchable_attributes(["name", "bio", "roles"])
        .with_filterable_attributes(["roles"])
        .with_sortable_attributes(["name"])
        .with_ranking_rules(["words", "typo", "proximity", "attribute", "exactness"])
}