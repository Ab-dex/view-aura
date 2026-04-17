--
-- PostgreSQL database dump
--

\restrict 7T2U6FbtdqjHAlV5l2BYXm38ANLUw0tBH8qNt52fHJ7GrpdQniFNc32xiIRUfEV

-- Dumped from database version 16.13
-- Dumped by pg_dump version 18.3 (Homebrew)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: public; Type: SCHEMA; Schema: -; Owner: -
--

-- *not* creating schema, since initdb creates it


--
-- Name: SCHEMA public; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON SCHEMA public IS '';


--
-- Name: citext; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS citext WITH SCHEMA public;


--
-- Name: EXTENSION citext; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION citext IS 'data type for case-insensitive character strings';


--
-- Name: pg_trgm; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;


--
-- Name: EXTENSION pg_trgm; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pg_trgm IS 'text similarity measurement and index searching based on trigrams';


--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- Name: EXTENSION pgcrypto; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';


--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: credits; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.credits (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    movie_id uuid NOT NULL,
    person_id uuid NOT NULL,
    role text NOT NULL,
    "character" text,
    billing_order integer DEFAULT 999 NOT NULL
);


--
-- Name: custom_list_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.custom_list_items (
    list_id uuid NOT NULL,
    movie_id uuid NOT NULL,
    "position" integer DEFAULT 0 NOT NULL,
    note text,
    added_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: custom_lists; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.custom_lists (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    name text NOT NULL,
    description text,
    is_public boolean DEFAULT true NOT NULL,
    cover_url text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: filming_locations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.filming_locations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    movie_id uuid NOT NULL,
    name text NOT NULL,
    latitude numeric(9,6),
    longitude numeric(9,6),
    description text
);


--
-- Name: follows; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.follows (
    follower_id uuid NOT NULL,
    followee_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT follows_check CHECK ((follower_id <> followee_id))
);


--
-- Name: genres; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.genres (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    slug text NOT NULL,
    description text
);


--
-- Name: invoices; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.invoices (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid,
    subscription_id uuid,
    stripe_invoice_id text NOT NULL,
    amount_cents bigint NOT NULL,
    currency text DEFAULT 'usd'::text NOT NULL,
    status text NOT NULL,
    paid_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT invoices_amount_cents_check CHECK ((amount_cents >= 0)),
    CONSTRAINT invoices_status_check CHECK ((status = ANY (ARRAY['paid'::text, 'open'::text, 'void'::text])))
);


--
-- Name: list_collaborators; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.list_collaborators (
    list_id uuid NOT NULL,
    user_id uuid NOT NULL,
    access text DEFAULT 'read'::text NOT NULL,
    invited_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT list_collaborators_access_check CHECK ((access = ANY (ARRAY['read'::text, 'write'::text])))
);


--
-- Name: media_assets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.media_assets (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    upload_id uuid NOT NULL,
    user_id uuid NOT NULL,
    movie_id uuid,
    storage_key text NOT NULL,
    content_type text NOT NULL,
    size_bytes bigint NOT NULL,
    status text DEFAULT 'uploading'::text NOT NULL,
    failure_reason text,
    source_info jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT media_assets_size_bytes_check CHECK ((size_bytes > 0)),
    CONSTRAINT media_assets_status_check CHECK ((status = ANY (ARRAY['uploading'::text, 'processing'::text, 'ready'::text, 'failed'::text, 'moderated'::text])))
);


--
-- Name: moderation_appeals; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.moderation_appeals (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    case_id uuid NOT NULL,
    appellant_id uuid NOT NULL,
    statement text NOT NULL,
    outcome text,
    decided_by text,
    decided_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT moderation_appeals_outcome_check CHECK ((outcome = ANY (ARRAY['upheld'::text, 'overturned'::text])))
);


--
-- Name: moderation_cases; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.moderation_cases (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    content_type text NOT NULL,
    content_id text NOT NULL,
    owner_id uuid,
    submitted_by text NOT NULL,
    trigger_reason text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    ai_signals text[] DEFAULT '{}'::text[] NOT NULL,
    ai_confidence jsonb,
    ai_recommendation text,
    decision text,
    decided_by text,
    reason text,
    decided_at timestamp with time zone,
    locked_by text,
    locked_until timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT moderation_cases_ai_recommendation_check CHECK ((ai_recommendation = ANY (ARRAY['approve'::text, 'reject'::text, 'escalate_to_human'::text]))),
    CONSTRAINT moderation_cases_content_type_check CHECK ((content_type = ANY (ARRAY['review'::text, 'upload'::text, 'post'::text, 'profile'::text, 'comment'::text]))),
    CONSTRAINT moderation_cases_decision_check CHECK ((decision = ANY (ARRAY['approved'::text, 'rejected'::text, 'escalated'::text]))),
    CONSTRAINT moderation_cases_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'ai_reviewed'::text, 'in_review'::text, 'decided'::text, 'appealed'::text]))),
    CONSTRAINT moderation_cases_trigger_reason_check CHECK ((trigger_reason = ANY (ARRAY['auto_screen'::text, 'user_report'::text, 'admin_flag'::text, 'ai_flag'::text])))
);


--
-- Name: movies; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.movies (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    title text NOT NULL,
    original_title text,
    slug text NOT NULL,
    synopsis text,
    tagline text,
    release_date date,
    runtime_mins integer,
    content_rating text,
    status text DEFAULT 'draft'::text NOT NULL,
    original_lang text DEFAULT 'en'::text NOT NULL,
    countries text[] DEFAULT '{}'::text[] NOT NULL,
    genres text[] DEFAULT '{}'::text[] NOT NULL,
    poster_url text,
    backdrop_url text,
    trailer_url text,
    imdb_id text,
    tmdb_id integer,
    avg_rating numeric(4,2) DEFAULT 0 NOT NULL,
    rating_count integer DEFAULT 0 NOT NULL,
    popularity_score numeric(10,4) DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT movies_runtime_mins_check CHECK ((runtime_mins > 0)),
    CONSTRAINT movies_status_check CHECK ((status = ANY (ARRAY['draft'::text, 'published'::text, 'archived'::text])))
);


--
-- Name: ratings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ratings (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    movie_id uuid NOT NULL,
    user_id uuid NOT NULL,
    overall numeric(3,1) NOT NULL,
    acting numeric(3,1),
    direction numeric(3,1),
    writing numeric(3,1),
    cinematography numeric(3,1),
    soundtrack numeric(3,1),
    reaction text,
    is_verified boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT ratings_acting_check CHECK (((acting >= (1)::numeric) AND (acting <= (5)::numeric))),
    CONSTRAINT ratings_cinematography_check CHECK (((cinematography >= (1)::numeric) AND (cinematography <= (5)::numeric))),
    CONSTRAINT ratings_direction_check CHECK (((direction >= (1)::numeric) AND (direction <= (5)::numeric))),
    CONSTRAINT ratings_overall_check CHECK (((overall >= (1)::numeric) AND (overall <= (5)::numeric))),
    CONSTRAINT ratings_soundtrack_check CHECK (((soundtrack >= (1)::numeric) AND (soundtrack <= (5)::numeric))),
    CONSTRAINT ratings_writing_check CHECK (((writing >= (1)::numeric) AND (writing <= (5)::numeric)))
);


--
-- Name: mv_movie_rating_agg; Type: MATERIALIZED VIEW; Schema: public; Owner: -
--

CREATE MATERIALIZED VIEW public.mv_movie_rating_agg AS
 SELECT movie_id,
    round(avg(overall), 2) AS avg_overall,
    round(avg(acting), 2) AS avg_acting,
    round(avg(direction), 2) AS avg_direction,
    round(avg(writing), 2) AS avg_writing,
    round(avg(cinematography), 2) AS avg_cinematography,
    round(avg(soundtrack), 2) AS avg_soundtrack,
    count(*) AS total_ratings,
    count(*) FILTER (WHERE ((round(overall))::integer = 1)) AS dist_1,
    count(*) FILTER (WHERE ((round(overall))::integer = 2)) AS dist_2,
    count(*) FILTER (WHERE ((round(overall))::integer = 3)) AS dist_3,
    count(*) FILTER (WHERE ((round(overall))::integer = 4)) AS dist_4,
    count(*) FILTER (WHERE ((round(overall))::integer = 5)) AS dist_5,
    sum((overall * exp((('-0.001'::numeric * EXTRACT(epoch FROM (CURRENT_TIMESTAMP - created_at))) / (86400)::numeric)))) AS weighted_score,
    max(updated_at) AS last_updated
   FROM public.ratings
  GROUP BY movie_id
  WITH NO DATA;


--
-- Name: mv_genre_leaderboard; Type: MATERIALIZED VIEW; Schema: public; Owner: -
--

CREATE MATERIALIZED VIEW public.mv_genre_leaderboard AS
 SELECT g.genre,
    m.id AS movie_id,
    m.title,
    m.poster_url,
    r.weighted_score,
    r.total_ratings,
    r.avg_overall,
    row_number() OVER (PARTITION BY g.genre ORDER BY r.weighted_score DESC) AS rank
   FROM ((public.mv_movie_rating_agg r
     JOIN public.movies m ON ((m.id = r.movie_id)))
     CROSS JOIN LATERAL unnest(m.genres) g(genre))
  WHERE ((m.status = 'published'::text) AND (r.total_ratings >= 10))
  WITH NO DATA;


--
-- Name: notifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notifications (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    type text NOT NULL,
    title text NOT NULL,
    body text NOT NULL,
    data jsonb,
    channel text NOT NULL,
    is_read boolean DEFAULT false NOT NULL,
    sent_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT notifications_channel_check CHECK ((channel = ANY (ARRAY['push'::text, 'email'::text, 'sms'::text, 'in_app'::text])))
);


--
-- Name: permissions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.permissions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: persons; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.persons (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    slug text NOT NULL,
    birth_date date,
    death_date date,
    birthplace text,
    bio text,
    profile_url text,
    imdb_id text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: push_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.push_tokens (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    session_id uuid,
    provider text NOT NULL,
    token text NOT NULL,
    device_type text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    last_used_at timestamp with time zone,
    CONSTRAINT push_tokens_device_type_check CHECK ((device_type = ANY (ARRAY['ios'::text, 'android'::text, 'web'::text]))),
    CONSTRAINT push_tokens_provider_check CHECK ((provider = ANY (ARRAY['fcm'::text, 'apns'::text])))
);


--
-- Name: review_reactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.review_reactions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    review_id uuid NOT NULL,
    user_id uuid NOT NULL,
    type text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT review_reactions_type_check CHECK ((type = ANY (ARRAY['like'::text, 'helpful'::text, 'insightful'::text, 'funny'::text])))
);


--
-- Name: reviews; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reviews (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    movie_id uuid NOT NULL,
    user_id uuid NOT NULL,
    type text NOT NULL,
    body text,
    video_url text,
    is_spoiler boolean DEFAULT false NOT NULL,
    is_published boolean DEFAULT false NOT NULL,
    like_count integer DEFAULT 0 NOT NULL,
    helpful_count integer DEFAULT 0 NOT NULL,
    insightful_count integer DEFAULT 0 NOT NULL,
    funny_count integer DEFAULT 0 NOT NULL,
    credibility_score numeric(5,4) DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT reviews_type_check CHECK ((type = ANY (ARRAY['short'::text, 'long_form'::text, 'spoiler'::text, 'video'::text])))
);


--
-- Name: role_permissions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.role_permissions (
    role_id uuid NOT NULL,
    permission_id uuid NOT NULL
);


--
-- Name: roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.roles (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    description text,
    is_system boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version bigint NOT NULL,
    dirty boolean NOT NULL
);


--
-- Name: screener_licenses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.screener_licenses (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    movie_id uuid NOT NULL,
    amount_cents bigint NOT NULL,
    currency text DEFAULT 'usd'::text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT screener_licenses_amount_cents_check CHECK ((amount_cents >= 0))
);


--
-- Name: streaming_links; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.streaming_links (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    movie_id uuid NOT NULL,
    provider text NOT NULL,
    link_url text NOT NULL,
    access_type text NOT NULL,
    price_cents integer,
    region text DEFAULT ''::text NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT streaming_links_access_type_check CHECK ((access_type = ANY (ARRAY['stream'::text, 'rent'::text, 'buy'::text])))
);


--
-- Name: subscriptions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.subscriptions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    plan text NOT NULL,
    status text NOT NULL,
    stripe_sub_id text NOT NULL,
    stripe_customer_id text NOT NULL,
    current_period_end timestamp with time zone NOT NULL,
    cancel_at_end boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT subscriptions_plan_check CHECK ((plan = ANY (ARRAY['free'::text, 'pro'::text, 'studio'::text]))),
    CONSTRAINT subscriptions_status_check CHECK ((status = ANY (ARRAY['active'::text, 'past_due'::text, 'cancelled'::text, 'trialing'::text])))
);


--
-- Name: user_auth_providers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_auth_providers (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    provider text NOT NULL,
    provider_user_id text NOT NULL,
    access_token text,
    refresh_token text,
    linked_at timestamp with time zone DEFAULT now() NOT NULL,
    last_used_at timestamp with time zone,
    CONSTRAINT user_auth_providers_provider_check CHECK ((provider = ANY (ARRAY['email'::text, 'google'::text, 'apple'::text, 'github'::text])))
);


--
-- Name: user_preferences; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_preferences (
    user_id uuid NOT NULL,
    preferred_genres text[] DEFAULT '{}'::text[] NOT NULL,
    disliked_genres text[] DEFAULT '{}'::text[] NOT NULL,
    preferred_languages text[] DEFAULT '{}'::text[] NOT NULL,
    adult_content boolean DEFAULT false NOT NULL,
    dark_mode boolean DEFAULT true NOT NULL,
    autoplay_trailers boolean DEFAULT true NOT NULL,
    dnd_start_hour smallint,
    dnd_end_hour smallint,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT user_preferences_dnd_end_hour_check CHECK (((dnd_end_hour >= 0) AND (dnd_end_hour <= 23))),
    CONSTRAINT user_preferences_dnd_start_hour_check CHECK (((dnd_start_hour >= 0) AND (dnd_start_hour <= 23)))
);


--
-- Name: user_profiles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_profiles (
    user_id uuid NOT NULL,
    avatar_url text,
    banner_url text,
    bio text,
    website text,
    birthdate date,
    gender text,
    visibility text DEFAULT 'public'::text NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT user_profiles_visibility_check CHECK ((visibility = ANY (ARRAY['public'::text, 'private'::text, 'friends'::text])))
);


--
-- Name: user_security; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_security (
    user_id uuid NOT NULL,
    failed_login_count integer DEFAULT 0 NOT NULL,
    last_failed_at timestamp with time zone,
    locked_until timestamp with time zone,
    risk_score numeric(5,4) DEFAULT 0 NOT NULL,
    two_factor_enabled boolean DEFAULT false NOT NULL,
    two_factor_secret text,
    last_login_at timestamp with time zone,
    last_login_ip inet,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_sessions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    device_id text,
    ip_address inet,
    user_agent text,
    refresh_token_hash text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    revoked_at timestamp with time zone
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    email public.citext NOT NULL,
    email_verified boolean DEFAULT false NOT NULL,
    username text,
    display_name text NOT NULL,
    role_id uuid NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    locale text DEFAULT 'en'::text NOT NULL,
    timezone text DEFAULT 'UTC'::text NOT NULL,
    country text,
    is_deleted boolean DEFAULT false NOT NULL,
    deleted_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT users_status_check CHECK ((status = ANY (ARRAY['active'::text, 'suspended'::text, 'deleted'::text, 'pending'::text])))
);


--
-- Name: watchlist_entries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.watchlist_entries (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    movie_id uuid NOT NULL,
    status text NOT NULL,
    progress integer DEFAULT 0 NOT NULL,
    added_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT watchlist_entries_progress_check CHECK ((progress >= 0)),
    CONSTRAINT watchlist_entries_status_check CHECK ((status = ANY (ARRAY['to_watch'::text, 'watching'::text, 'watched'::text, 'dropped'::text])))
);


--
-- Name: credits credits_movie_id_person_id_role_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.credits
    ADD CONSTRAINT credits_movie_id_person_id_role_key UNIQUE (movie_id, person_id, role);


--
-- Name: credits credits_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.credits
    ADD CONSTRAINT credits_pkey PRIMARY KEY (id);


--
-- Name: custom_list_items custom_list_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.custom_list_items
    ADD CONSTRAINT custom_list_items_pkey PRIMARY KEY (list_id, movie_id);


--
-- Name: custom_lists custom_lists_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.custom_lists
    ADD CONSTRAINT custom_lists_pkey PRIMARY KEY (id);


--
-- Name: filming_locations filming_locations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.filming_locations
    ADD CONSTRAINT filming_locations_pkey PRIMARY KEY (id);


--
-- Name: follows follows_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.follows
    ADD CONSTRAINT follows_pkey PRIMARY KEY (follower_id, followee_id);


--
-- Name: genres genres_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.genres
    ADD CONSTRAINT genres_name_key UNIQUE (name);


--
-- Name: genres genres_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.genres
    ADD CONSTRAINT genres_pkey PRIMARY KEY (id);


--
-- Name: genres genres_slug_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.genres
    ADD CONSTRAINT genres_slug_key UNIQUE (slug);


--
-- Name: invoices invoices_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT invoices_pkey PRIMARY KEY (id);


--
-- Name: invoices invoices_stripe_invoice_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT invoices_stripe_invoice_id_key UNIQUE (stripe_invoice_id);


--
-- Name: list_collaborators list_collaborators_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.list_collaborators
    ADD CONSTRAINT list_collaborators_pkey PRIMARY KEY (list_id, user_id);


--
-- Name: media_assets media_assets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_assets
    ADD CONSTRAINT media_assets_pkey PRIMARY KEY (id);


--
-- Name: media_assets media_assets_upload_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_assets
    ADD CONSTRAINT media_assets_upload_id_key UNIQUE (upload_id);


--
-- Name: moderation_appeals moderation_appeals_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.moderation_appeals
    ADD CONSTRAINT moderation_appeals_pkey PRIMARY KEY (id);


--
-- Name: moderation_cases moderation_cases_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.moderation_cases
    ADD CONSTRAINT moderation_cases_pkey PRIMARY KEY (id);


--
-- Name: movies movies_imdb_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.movies
    ADD CONSTRAINT movies_imdb_id_key UNIQUE (imdb_id);


--
-- Name: movies movies_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.movies
    ADD CONSTRAINT movies_pkey PRIMARY KEY (id);


--
-- Name: movies movies_slug_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.movies
    ADD CONSTRAINT movies_slug_key UNIQUE (slug);


--
-- Name: movies movies_tmdb_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.movies
    ADD CONSTRAINT movies_tmdb_id_key UNIQUE (tmdb_id);


--
-- Name: notifications notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_pkey PRIMARY KEY (id);


--
-- Name: permissions permissions_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permissions
    ADD CONSTRAINT permissions_name_key UNIQUE (name);


--
-- Name: permissions permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.permissions
    ADD CONSTRAINT permissions_pkey PRIMARY KEY (id);


--
-- Name: persons persons_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.persons
    ADD CONSTRAINT persons_pkey PRIMARY KEY (id);


--
-- Name: persons persons_slug_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.persons
    ADD CONSTRAINT persons_slug_key UNIQUE (slug);


--
-- Name: push_tokens push_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.push_tokens
    ADD CONSTRAINT push_tokens_pkey PRIMARY KEY (id);


--
-- Name: push_tokens push_tokens_token_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.push_tokens
    ADD CONSTRAINT push_tokens_token_key UNIQUE (token);


--
-- Name: ratings ratings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ratings
    ADD CONSTRAINT ratings_pkey PRIMARY KEY (id);


--
-- Name: ratings ratings_user_id_movie_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ratings
    ADD CONSTRAINT ratings_user_id_movie_id_key UNIQUE (user_id, movie_id);


--
-- Name: review_reactions review_reactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.review_reactions
    ADD CONSTRAINT review_reactions_pkey PRIMARY KEY (id);


--
-- Name: review_reactions review_reactions_review_id_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.review_reactions
    ADD CONSTRAINT review_reactions_review_id_user_id_key UNIQUE (review_id, user_id);


--
-- Name: reviews reviews_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT reviews_pkey PRIMARY KEY (id);


--
-- Name: role_permissions role_permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.role_permissions
    ADD CONSTRAINT role_permissions_pkey PRIMARY KEY (role_id, permission_id);


--
-- Name: roles roles_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roles
    ADD CONSTRAINT roles_name_key UNIQUE (name);


--
-- Name: roles roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roles
    ADD CONSTRAINT roles_pkey PRIMARY KEY (id);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: screener_licenses screener_licenses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.screener_licenses
    ADD CONSTRAINT screener_licenses_pkey PRIMARY KEY (id);


--
-- Name: streaming_links streaming_links_movie_id_provider_region_access_type_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.streaming_links
    ADD CONSTRAINT streaming_links_movie_id_provider_region_access_type_key UNIQUE (movie_id, provider, region, access_type);


--
-- Name: streaming_links streaming_links_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.streaming_links
    ADD CONSTRAINT streaming_links_pkey PRIMARY KEY (id);


--
-- Name: subscriptions subscriptions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscriptions
    ADD CONSTRAINT subscriptions_pkey PRIMARY KEY (id);


--
-- Name: subscriptions subscriptions_stripe_sub_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscriptions
    ADD CONSTRAINT subscriptions_stripe_sub_id_key UNIQUE (stripe_sub_id);


--
-- Name: user_auth_providers user_auth_providers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_auth_providers
    ADD CONSTRAINT user_auth_providers_pkey PRIMARY KEY (id);


--
-- Name: user_auth_providers user_auth_providers_provider_provider_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_auth_providers
    ADD CONSTRAINT user_auth_providers_provider_provider_user_id_key UNIQUE (provider, provider_user_id);


--
-- Name: user_preferences user_preferences_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_preferences
    ADD CONSTRAINT user_preferences_pkey PRIMARY KEY (user_id);


--
-- Name: user_profiles user_profiles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_profiles
    ADD CONSTRAINT user_profiles_pkey PRIMARY KEY (user_id);


--
-- Name: user_security user_security_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_security
    ADD CONSTRAINT user_security_pkey PRIMARY KEY (user_id);


--
-- Name: user_sessions user_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_sessions
    ADD CONSTRAINT user_sessions_pkey PRIMARY KEY (id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: users users_username_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_username_key UNIQUE (username);


--
-- Name: watchlist_entries watchlist_entries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.watchlist_entries
    ADD CONSTRAINT watchlist_entries_pkey PRIMARY KEY (id);


--
-- Name: watchlist_entries watchlist_entries_user_id_movie_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.watchlist_entries
    ADD CONSTRAINT watchlist_entries_user_id_movie_id_key UNIQUE (user_id, movie_id);


--
-- Name: idx_auth_providers_provider; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_auth_providers_provider ON public.user_auth_providers USING btree (provider, provider_user_id);


--
-- Name: idx_auth_providers_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_auth_providers_user ON public.user_auth_providers USING btree (user_id);


--
-- Name: idx_credits_movie; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_credits_movie ON public.credits USING btree (movie_id);


--
-- Name: idx_credits_person; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_credits_person ON public.credits USING btree (person_id);


--
-- Name: idx_custom_lists_public; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_custom_lists_public ON public.custom_lists USING btree (created_at DESC) WHERE (is_public = true);


--
-- Name: idx_custom_lists_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_custom_lists_user ON public.custom_lists USING btree (user_id);


--
-- Name: idx_filming_movie; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_filming_movie ON public.filming_locations USING btree (movie_id);


--
-- Name: idx_follows_followee; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_follows_followee ON public.follows USING btree (followee_id);


--
-- Name: idx_follows_followee_count; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_follows_followee_count ON public.follows USING btree (followee_id) INCLUDE (follower_id);


--
-- Name: idx_follows_follower; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_follows_follower ON public.follows USING btree (follower_id);


--
-- Name: idx_invoices_stripe; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_invoices_stripe ON public.invoices USING btree (stripe_invoice_id);


--
-- Name: idx_invoices_subscription; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_invoices_subscription ON public.invoices USING btree (subscription_id);


--
-- Name: idx_invoices_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_invoices_user ON public.invoices USING btree (user_id);


--
-- Name: idx_licenses_expiry; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_licenses_expiry ON public.screener_licenses USING btree (expires_at);


--
-- Name: idx_licenses_user_movie; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_licenses_user_movie ON public.screener_licenses USING btree (user_id, movie_id);


--
-- Name: idx_list_collaborators_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_list_collaborators_user ON public.list_collaborators USING btree (user_id);


--
-- Name: idx_list_items_list; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_list_items_list ON public.custom_list_items USING btree (list_id, "position");


--
-- Name: idx_list_items_movie; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_list_items_movie ON public.custom_list_items USING btree (movie_id);


--
-- Name: idx_media_assets_movie; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_media_assets_movie ON public.media_assets USING btree (movie_id) WHERE (movie_id IS NOT NULL);


--
-- Name: idx_media_assets_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_media_assets_status ON public.media_assets USING btree (status) WHERE (status <> 'ready'::text);


--
-- Name: idx_media_assets_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_media_assets_user ON public.media_assets USING btree (user_id);


--
-- Name: idx_mod_appeals_appellant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_mod_appeals_appellant ON public.moderation_appeals USING btree (appellant_id);


--
-- Name: idx_mod_appeals_case; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_mod_appeals_case ON public.moderation_appeals USING btree (case_id);


--
-- Name: idx_mod_cases_content; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_mod_cases_content ON public.moderation_cases USING btree (content_type, content_id);


--
-- Name: idx_mod_cases_owner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_mod_cases_owner ON public.moderation_cases USING btree (owner_id);


--
-- Name: idx_mod_cases_queue; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_mod_cases_queue ON public.moderation_cases USING btree (created_at) WHERE (status = ANY (ARRAY['pending'::text, 'ai_reviewed'::text]));


--
-- Name: idx_mod_cases_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_mod_cases_status ON public.moderation_cases USING btree (status);


--
-- Name: idx_movies_countries; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_movies_countries ON public.movies USING gin (countries);


--
-- Name: idx_movies_genres; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_movies_genres ON public.movies USING gin (genres);


--
-- Name: idx_movies_imdb; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_movies_imdb ON public.movies USING btree (imdb_id) WHERE (imdb_id IS NOT NULL);


--
-- Name: idx_movies_popularity; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_movies_popularity ON public.movies USING btree (popularity_score DESC, avg_rating DESC) WHERE (status = 'published'::text);


--
-- Name: idx_movies_release; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_movies_release ON public.movies USING btree (release_date DESC) WHERE (status = 'published'::text);


--
-- Name: idx_movies_slug; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_movies_slug ON public.movies USING btree (slug);


--
-- Name: idx_movies_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_movies_status ON public.movies USING btree (status);


--
-- Name: idx_movies_tmdb; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_movies_tmdb ON public.movies USING btree (tmdb_id) WHERE (tmdb_id IS NOT NULL);


--
-- Name: idx_movies_trgm_title; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_movies_trgm_title ON public.movies USING gin (title public.gin_trgm_ops);


--
-- Name: idx_mv_leaderboard_genre; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_mv_leaderboard_genre ON public.mv_genre_leaderboard USING btree (genre, rank);


--
-- Name: idx_mv_rating_agg_movie; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_mv_rating_agg_movie ON public.mv_movie_rating_agg USING btree (movie_id);


--
-- Name: idx_notifications_unread; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_unread ON public.notifications USING btree (user_id) WHERE ((is_read = false) AND (channel = 'in_app'::text));


--
-- Name: idx_notifications_unsent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_unsent ON public.notifications USING btree (created_at) WHERE (sent_at IS NULL);


--
-- Name: idx_notifications_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_user ON public.notifications USING btree (user_id, created_at DESC);


--
-- Name: idx_persons_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_persons_name ON public.persons USING gin (name public.gin_trgm_ops);


--
-- Name: idx_push_tokens_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_push_tokens_user ON public.push_tokens USING btree (user_id);


--
-- Name: idx_ratings_movie; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ratings_movie ON public.ratings USING btree (movie_id);


--
-- Name: idx_ratings_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ratings_user ON public.ratings USING btree (user_id);


--
-- Name: idx_ratings_verified; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ratings_verified ON public.ratings USING btree (movie_id, overall) WHERE (is_verified = true);


--
-- Name: idx_review_reactions_review; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_review_reactions_review ON public.review_reactions USING btree (review_id);


--
-- Name: idx_reviews_movie; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reviews_movie ON public.reviews USING btree (movie_id, created_at DESC) WHERE (is_published = true);


--
-- Name: idx_reviews_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reviews_type ON public.reviews USING btree (movie_id, type) WHERE (is_published = true);


--
-- Name: idx_reviews_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reviews_user ON public.reviews USING btree (user_id);


--
-- Name: idx_sessions_exp; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sessions_exp ON public.user_sessions USING btree (expires_at) WHERE (revoked_at IS NULL);


--
-- Name: idx_sessions_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sessions_hash ON public.user_sessions USING btree (refresh_token_hash);


--
-- Name: idx_sessions_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_sessions_user ON public.user_sessions USING btree (user_id);


--
-- Name: idx_streaming_movie; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_streaming_movie ON public.streaming_links USING btree (movie_id);


--
-- Name: idx_streaming_region; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_streaming_region ON public.streaming_links USING btree (movie_id, region);


--
-- Name: idx_subscriptions_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_subscriptions_status ON public.subscriptions USING btree (status);


--
-- Name: idx_subscriptions_stripe; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_subscriptions_stripe ON public.subscriptions USING btree (stripe_sub_id);


--
-- Name: idx_subscriptions_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_subscriptions_user ON public.subscriptions USING btree (user_id);


--
-- Name: idx_users_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_created_at ON public.users USING btree (created_at DESC);


--
-- Name: idx_users_email; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_email ON public.users USING btree (email);


--
-- Name: idx_users_role; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_role ON public.users USING btree (role_id);


--
-- Name: idx_users_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_status ON public.users USING btree (status) WHERE (is_deleted = false);


--
-- Name: idx_users_username; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_username ON public.users USING btree (username) WHERE (username IS NOT NULL);


--
-- Name: idx_watchlist_movie; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_watchlist_movie ON public.watchlist_entries USING btree (movie_id);


--
-- Name: idx_watchlist_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_watchlist_user ON public.watchlist_entries USING btree (user_id);


--
-- Name: idx_watchlist_user_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_watchlist_user_status ON public.watchlist_entries USING btree (user_id, status);


--
-- Name: credits credits_movie_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.credits
    ADD CONSTRAINT credits_movie_id_fkey FOREIGN KEY (movie_id) REFERENCES public.movies(id) ON DELETE CASCADE;


--
-- Name: credits credits_person_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.credits
    ADD CONSTRAINT credits_person_id_fkey FOREIGN KEY (person_id) REFERENCES public.persons(id) ON DELETE CASCADE;


--
-- Name: custom_list_items custom_list_items_list_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.custom_list_items
    ADD CONSTRAINT custom_list_items_list_id_fkey FOREIGN KEY (list_id) REFERENCES public.custom_lists(id) ON DELETE CASCADE;


--
-- Name: custom_list_items custom_list_items_movie_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.custom_list_items
    ADD CONSTRAINT custom_list_items_movie_id_fkey FOREIGN KEY (movie_id) REFERENCES public.movies(id) ON DELETE CASCADE;


--
-- Name: custom_lists custom_lists_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.custom_lists
    ADD CONSTRAINT custom_lists_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: filming_locations filming_locations_movie_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.filming_locations
    ADD CONSTRAINT filming_locations_movie_id_fkey FOREIGN KEY (movie_id) REFERENCES public.movies(id) ON DELETE CASCADE;


--
-- Name: follows follows_followee_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.follows
    ADD CONSTRAINT follows_followee_id_fkey FOREIGN KEY (followee_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: follows follows_follower_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.follows
    ADD CONSTRAINT follows_follower_id_fkey FOREIGN KEY (follower_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: invoices invoices_subscription_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT invoices_subscription_id_fkey FOREIGN KEY (subscription_id) REFERENCES public.subscriptions(id) ON DELETE SET NULL;


--
-- Name: invoices invoices_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT invoices_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: list_collaborators list_collaborators_list_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.list_collaborators
    ADD CONSTRAINT list_collaborators_list_id_fkey FOREIGN KEY (list_id) REFERENCES public.custom_lists(id) ON DELETE CASCADE;


--
-- Name: list_collaborators list_collaborators_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.list_collaborators
    ADD CONSTRAINT list_collaborators_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: media_assets media_assets_movie_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_assets
    ADD CONSTRAINT media_assets_movie_id_fkey FOREIGN KEY (movie_id) REFERENCES public.movies(id) ON DELETE SET NULL;


--
-- Name: media_assets media_assets_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_assets
    ADD CONSTRAINT media_assets_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: moderation_appeals moderation_appeals_appellant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.moderation_appeals
    ADD CONSTRAINT moderation_appeals_appellant_id_fkey FOREIGN KEY (appellant_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: moderation_appeals moderation_appeals_case_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.moderation_appeals
    ADD CONSTRAINT moderation_appeals_case_id_fkey FOREIGN KEY (case_id) REFERENCES public.moderation_cases(id) ON DELETE CASCADE;


--
-- Name: moderation_cases moderation_cases_owner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.moderation_cases
    ADD CONSTRAINT moderation_cases_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: notifications notifications_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: push_tokens push_tokens_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.push_tokens
    ADD CONSTRAINT push_tokens_session_id_fkey FOREIGN KEY (session_id) REFERENCES public.user_sessions(id) ON DELETE SET NULL;


--
-- Name: push_tokens push_tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.push_tokens
    ADD CONSTRAINT push_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: ratings ratings_movie_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ratings
    ADD CONSTRAINT ratings_movie_id_fkey FOREIGN KEY (movie_id) REFERENCES public.movies(id) ON DELETE CASCADE;


--
-- Name: ratings ratings_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ratings
    ADD CONSTRAINT ratings_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: review_reactions review_reactions_review_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.review_reactions
    ADD CONSTRAINT review_reactions_review_id_fkey FOREIGN KEY (review_id) REFERENCES public.reviews(id) ON DELETE CASCADE;


--
-- Name: review_reactions review_reactions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.review_reactions
    ADD CONSTRAINT review_reactions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: reviews reviews_movie_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT reviews_movie_id_fkey FOREIGN KEY (movie_id) REFERENCES public.movies(id) ON DELETE CASCADE;


--
-- Name: reviews reviews_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reviews
    ADD CONSTRAINT reviews_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: role_permissions role_permissions_permission_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.role_permissions
    ADD CONSTRAINT role_permissions_permission_id_fkey FOREIGN KEY (permission_id) REFERENCES public.permissions(id) ON DELETE CASCADE;


--
-- Name: role_permissions role_permissions_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.role_permissions
    ADD CONSTRAINT role_permissions_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.roles(id) ON DELETE CASCADE;


--
-- Name: screener_licenses screener_licenses_movie_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.screener_licenses
    ADD CONSTRAINT screener_licenses_movie_id_fkey FOREIGN KEY (movie_id) REFERENCES public.movies(id) ON DELETE CASCADE;


--
-- Name: screener_licenses screener_licenses_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.screener_licenses
    ADD CONSTRAINT screener_licenses_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: streaming_links streaming_links_movie_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.streaming_links
    ADD CONSTRAINT streaming_links_movie_id_fkey FOREIGN KEY (movie_id) REFERENCES public.movies(id) ON DELETE CASCADE;


--
-- Name: subscriptions subscriptions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscriptions
    ADD CONSTRAINT subscriptions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_auth_providers user_auth_providers_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_auth_providers
    ADD CONSTRAINT user_auth_providers_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_preferences user_preferences_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_preferences
    ADD CONSTRAINT user_preferences_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_profiles user_profiles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_profiles
    ADD CONSTRAINT user_profiles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_security user_security_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_security
    ADD CONSTRAINT user_security_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_sessions user_sessions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_sessions
    ADD CONSTRAINT user_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: users users_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_role_id_fkey FOREIGN KEY (role_id) REFERENCES public.roles(id);


--
-- Name: watchlist_entries watchlist_entries_movie_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.watchlist_entries
    ADD CONSTRAINT watchlist_entries_movie_id_fkey FOREIGN KEY (movie_id) REFERENCES public.movies(id) ON DELETE CASCADE;


--
-- Name: watchlist_entries watchlist_entries_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.watchlist_entries
    ADD CONSTRAINT watchlist_entries_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict 7T2U6FbtdqjHAlV5l2BYXm38ANLUw0tBH8qNt52fHJ7GrpdQniFNc32xiIRUfEV

