# ViewAura Product Requirements Document (PRD)

**Document status:** Proposed product specification\
**Product:** ViewAura\
**Source baseline:** `README.md` supplied with the project\
**Audience:** Product, engineering, design, QA, operations, content
partners\
**Version:** 2.0\
**Date:** 23 November 2026

------------------------------------------------------------------------

## 1. Executive summary

ViewAura is an independent-film distribution, audience and media-operations platform for small media houses, independent studios, filmmakers and the viewers who want to discover their work. It combines a film-specific publishing and distribution workflow with audience relationship tools, monetization, rights-aware streaming, secure media vaulting and a consumer discovery experience.

The product is not positioned to win by reproducing Netflix's catalogue scale, IMDb's breadth of metadata or Letterboxd's community maturity. Its wedge is to help a small media house do work that is painful or fragmented across existing providers: publish a film, control where and how it is distributed, reach and retain its audience, monetize different rights windows, manage screeners and assets, and understand commercial performance—without first needing a massive audience or an enterprise operations team.

The PRD therefore treats the following as connected product capabilities rather than isolated features:

1. **Studio operating surface:** Branded studio presence, title publishing, release scheduling, rights windows, pricing, promotion, audience management and revenue reporting.
2. **Distribution layer:** ViewAura discovery and streaming, embeddable players, private screening portals and later institutional/event distribution.
3. **Audience relationship layer:** Follows, consent-based notifications, community engagement, repeat-viewing signals and permitted audience/transaction exports.
4. **Media and rights vault:** Versioned originals and masters, subtitles, screeners, distribution packages, access controls, watermarking, retention and licensing records.
5. **Viewer experience:** Discovery, social film activity, reliable adaptive streaming, data-aware playback and licensed offline access.
6. **Lean operations:** Reuse the current modular architecture and introduce costly infrastructure only when measured demand or contractual requirements justify it.

Streaming, offline resilience and vaulting remain essential, but they are enabling capabilities. The defensible value is the connected workflow and the operational/commercial outcomes it gives small media businesses.

The supplied README describes a substantial active-development backend.
It does **not** establish that all listed capabilities are
production-ready. In particular, native mobile apps are a non-goal for
the current repository, DRM is not implemented, several satellite
services are scaffolds, some workflow activities are not wired, and
BFF/analytics/test coverage remain incomplete. This PRD treats the
requested streaming, offline and vault experiences as product
requirements to implement and validate, not as claims about existing
functionality.

## 2. Product vision

**Vision:** Become the operating and distribution platform that helps independent media businesses publish, protect, monetize and grow direct relationships with film audiences—while giving viewers a compelling place to discover and experience independent films.

**Positioning:** ViewAura is the independent film distribution and audience platform.

**Core promise to studios:** Publish once, distribute through multiple controlled channels, manage rights and screeners, understand audience behavior, and build a durable relationship with viewers without assembling a fragmented stack of tools.

**Core promise to viewers:** Discover distinctive films, follow the people and studios behind them, watch reliably and participate in film communities.

### 2.1 Product principles

-   **Studio value before marketplace scale:** A studio must get standalone operational value from publishing, rights, screeners, direct distribution and analytics even before ViewAura has a large consumer audience.
-   **Distribution is more than hosting:** A published title must connect rights, channels, audience, pricing, promotion, playback and reporting.
-   **Audience relationships are permissioned:** Studios can build direct relationships through follows, opt-in communications and permitted exports, but ViewAura does not claim ownership of viewers' personal data.
-   **One film identity:** Catalog, rights, distribution offers, reviews, availability, playback and social activity refer to one canonical title identity.
-   **Playback is a first-class product:** A playable title should have
    clear availability, entitlement, quality, subtitle and device
    information.
-   **Offline is deliberate, not accidental:** Users can prepare content
    and selected app data for connectivity gaps; offline actions
    reconcile safely later.
-   **Private means private:** Vault assets are private by default,
    access is explicit, and every sensitive action is auditable.
-   **Rights before playback:** A catalog entry, a streaming link, a
    subscription, a purchase and a download license are distinct
    concepts.
-   **Graceful degradation:** Discovery and playback should degrade
    predictably when optional dependencies fail.
-   **Cost follows usage:** Store originals once, package only needed
    renditions, cache aggressively, and use managed services until
    workload economics justify self-hosting.
-   **Modular before distributed:** Keep product-critical transactional
    domains in the Go modular monolith unless a measurable operational
    or scaling reason justifies extraction.

## 3. Problem statement

Small media houses often stitch together file storage, transfer links, video hosting, storefronts, ticketing/payment tools, mailing lists, social networks and spreadsheets. These tools rarely share a rights model, title identity, audience history or revenue view. The result is duplicated work, weak audience insight, fragile screener workflows, limited control over distribution windows and dependence on intermediaries for customer relationships.

At the same time, viewers discover and discuss films in places that are disconnected from the actual distribution channel. A streaming-only product also excludes users facing expensive data, intermittent connectivity, limited device storage or variable device capability.

ViewAura addresses the studio-side fragmentation first and uses the resulting catalogue and creator relationships to build a differentiated viewer marketplace. Its strategic risk is real: without a meaningful operational advantage for studios, it is merely a smaller streaming service competing against companies with better catalogues, infrastructure and budgets. Every major product investment must therefore map to a measurable studio or viewer pain point.
## 4. Users and personas

| Persona | Primary jobs | Key needs / value expected |
|---|---|---|
| Small media house / studio operator (primary buyer) | Publish and distribute a slate; manage rights, revenue, campaigns and collaborators | A low-operations distribution desk, clear costs, revenue visibility, control over windows and channels |
| Independent filmmaker / rights holder | Deliver films, trailers, subtitles and promotional assets; invite screeners | Secure versioned vault, resumable upload, private review, rights tracking and transparent release workflow |
| Distributor / sales agent | Manage territories, windows, offers and partner access | Rights-aware availability, offer controls, audit trails and performance reports |
| Viewer / film enthusiast | Discover, follow, watch, rate and discuss films | Distinctive catalogue, trustworthy availability, reliable playback, data controls and community |
| Screener recipient / reviewer | Access pre-release films securely | Invitation-only access, expiry, watermarking, playback controls and clear terms |
| Festival / institution / event organizer (later buyer) | Program titles for a bounded audience or event | Private screening rooms, access windows, audience limits and reporting |
| Moderator / catalog editor | Maintain safe content and accurate film records | Review queues, source attribution, case history and appeals |
| Platform operator | Keep the service secure, reliable and economically viable | Cost per title/viewing hour, observability, quotas, incident response and capacity controls |

**Initial ideal customer profile:** Small independent production companies, regional distributors and media houses with a catalogue they control, recurring releases, limited internal technical capacity and a current mix of storage/transfer, streaming, ticketing or social tools. Validate willingness to pay and workflow pain before expanding into broad consumer features.

## 5. Goals and non-goals

### 5.1 Goals

-   A studio can publish, distribute and monetize a title with a small operational burden.
-   Studio teams can manage collaborators, rights, release windows, offers, screeners and assets in one workspace.
-   Studios can distribute through ViewAura discovery, branded pages, embeds and private screening channels using one rights policy.
-   Studios can measure audience engagement, campaign conversion, revenue and delivery cost.
-   Viewers can follow studios/filmmakers and receive consent-based release updates.
-   Fast, accurate movie discovery, search and autocomplete.
-   Ratings, reviews, watchlists, custom lists, follows, feeds, threads
    and challenges.
-   Personalized recommendations from taste, content and mood signals.
-   Studio/creator upload, processing, moderation, publishing and asset
    management.
-   Adaptive bitrate video streaming with quality selection and
    subtitle/audio-track support.
-   Secure, entitlement-aware playback and download authorization.
-   Resumable uploads and recoverable processing workflows.
-   Offline access to eligible media and selected app data.
-   Safe offline mutations with conflict handling and idempotent
    synchronization.
-   Private media vaults with access policies, audit trails and
    retention rules.
-   Subscription, pay-per-view and screener licensing.
-   Cost and data-consumption controls that are understandable to users.
-   Measurable playback quality, upload success and product engagement.

### 5.2 Non-goals for the first delivery

-   Building a global studio-grade DRM licensing business before
    distribution rights require it.
-   Supporting every television, console, browser and mobile platform at
    launch.
-   Multi-region active-active writes.
-   Building a proprietary CDN.
-   Building an advanced AI scene-understanding pipeline before core
    playback, moderation and search are reliable.
-   Unlimited permanent downloads of licensed content.
-   Treating a catalog record or third-party availability link as proof
    that ViewAura owns streaming rights.
-   Making all user-generated content available offline.

### 5.3 Product wedge, moat and differentiation

ViewAura's moat is not a single feature. It is the compounding combination of:

1. **Creator workflow lock-in through real utility:** Asset versions, rights windows, release plans, screeners, distribution offers, campaign records and revenue history become a useful operating record for a studio. Data export and contractual portability remain supported; value comes from workflow quality, not hostage-taking.
2. **Audience relationship infrastructure:** Viewers can follow studios, filmmakers and titles; studios can send consent-based updates and understand engagement across ViewAura-owned channels. Provide export of permitted audience and transaction records subject to privacy law, user consent and platform policy.
3. **Multi-channel distribution:** One managed title can be published to the ViewAura marketplace, a studio-branded page, an embeddable player, a private screener portal and, later, festival/institutional channels. Each channel uses the same rights and entitlement policy.
4. **Film-specific media vault and rights operations:** Store masters, screeners, subtitles and derivatives with versions, access grants, expiry, watermarking, retention and territory/window metadata.
5. **Commercial and audience intelligence:** Connect campaign source, offer, conversion, playback completion, repeat viewing and revenue so studios can make better release decisions.
6. **Resilient access:** Data-saver playback, resumable transfers, offline metadata and licensed downloads address real constraints in emerging and bandwidth-variable markets.

The flywheel is: more useful studio operations attract more distinctive titles; titles attract viewers; viewer engagement produces better first-party, consented insight; that insight improves studio marketing and monetization; better studio outcomes attract more rights holders. This flywheel is a hypothesis to validate, not an assumed network effect.

**Competitive posture:** Do not attempt to outspend Netflix on content or delivery, IMDb on metadata breadth, or Letterboxd on social maturity. Study Vimeo and Uscreen as adjacent creator/video business products. Differentiate with film-house workflows: rights-aware release management, screeners, multi-channel distribution, audience portability, title-level economics and a unified media vault.

### 5.4 Product surfaces

- **ViewAura Studio:** Studio profile and branded storefront; title/asset management; upload and processing; release calendar; rights/windows; distribution offers; promotions; screener management; audience and revenue analytics; collaborator roles.
- **ViewAura Watch:** Viewer discovery, search, title pages, follows, lists, reviews, community, playback, watch history and licensed offline experience.
- **ViewAura Distribution:** Embeddable player, shareable title pages, private screening portal, partner/institutional access and versioned distribution APIs. This is a product surface and capability set, not necessarily a separate deployable service.

### 5.5 Business model and pricing hypotheses

Validate pricing with pilot studios. Candidate revenue streams:

- Studio subscription tiers based on active titles, storage, team seats, analytics depth and monthly processing allowance.
- Transaction fee or revenue share on rentals, purchases, subscriptions or paid premieres where ViewAura processes the transaction.
- Paid add-ons for high-volume storage, premium encoding, advanced watermarking, private screening capacity and institutional licensing tools.
- Viewer subscriptions or title-level rentals/purchases only where the rights agreement supports them.

Avoid charging for basic account creation or making the studio's core asset metadata inaccessible after plan downgrade. Clearly separate platform fees, payment processing, taxes, storage, encoding and delivery costs.

### 5.6 Product validation and north-star metric

The first validation cohort should be five small media houses or independent production companies, each onboarding at least one title. Interview them about distribution costs, revenue shares, release windows, screener workflows, audience data, current tools and willingness to pay.

**North-star metric:** Time to first studio revenue (studio onboarding → title ready → first distribution offer → first paying viewer → first successful payout).

Supporting metrics: active publishing studios, titles published per studio, studio retention, studio net revenue, viewer-to-paid conversion, repeat viewers per studio, campaign-attributed conversion, cost per processed source hour, cost per delivered viewing hour and support burden per title.

## 6. Product scope and release strategy

The release plan is reordered around the studio wedge. A viewer experience remains important, but ViewAura must first prove it can deliver standalone value to a small media house.

### Release 0 — Validate the wedge and close foundation gaps

- Interview and recruit five pilot studios; document current workflows and willingness to pay.
- Audit existing Go modules, schema, API contracts, security and operational status.
- Define canonical title, studio ownership, rights, offer, distribution channel and asset relationships.
- Complete auth/session security, roles, entitlements, rights checks, asset state model and quota accounting.
- Wire the Temporal upload workflow to real processing endpoints; verify private object storage.
- Establish one end-to-end HLS playback path and a basic studio title-publishing workflow.

**Exit condition:** A pilot studio can securely upload, process, rights-check and publish one title to a studio page; an authorized viewer can play it; operators can explain the title's storage/processing/delivery cost.

### Release 1 — Studio distribution MVP

- Studio accounts, team roles and branded studio pages.
- Title setup, asset library, upload/resume, processing status and publication checklist.
- Rights windows by territory, permitted use and channel.
- Simple distribution offers: free, rental/PPV and subscription access where rights permit.
- ViewAura title page and studio storefront; checkout/entitlement and payout ledger integration.
- Basic studio dashboard: views, starts, completion, paid conversion, gross/net revenue and processing/delivery cost.
- Shareable private screener links with expiry and audit; download disabled by default.

**Exit condition:** A studio can publish and monetize a title, understand basic performance and complete a payout without manual database intervention.

### Release 2 — Audience relationships and viewer experience

- Viewer discovery, search, catalog, ratings, reviews, watchlists, follows and community foundations.
- Follow studios/filmmakers and opt-in release notifications.
- Campaign links/promo codes and basic source-to-conversion attribution.
- Adaptive streaming, QoE telemetry, resume playback, subtitles and data-saver controls.
- Offline metadata and queued engagement sync; viewer offline downloads only for licensed titles on supported native clients.
- Studio audience insights with consent, suppression and export controls.

**Exit condition:** Studios can build a repeat audience through ViewAura and measure campaign engagement; viewers can discover and reliably watch titles.

### Release 3 — Differentiated media operations

- Versioned vault for originals, masters, subtitles, artwork, trailers, screeners and distribution packages.
- Granular collaborator permissions, access logs, expiry, retention and deletion workflows.
- Forensic watermarking for eligible screeners and recipient-level audit.
- Embeddable player and configurable external studio pages.
- Release scheduling, territories/windows, promo codes, bundles and campaign reporting.
- Permitted audience/transaction exports and studio portability tools.

### Release 4 — Institutional distribution and compounding intelligence

- Festival, campus, community cinema and corporate screening workflows.
- Multi-title bundles, event access, group entitlements and institution-level reporting.
- Licensed local caching/community distribution only after rights, security and unit economics are validated.
- Advanced recommendations, localization/subtitle workflows and cross-studio discovery.
- Multi-CDN, advanced DRM and high-scale infrastructure only when contractual or measured requirements justify them.

**Explicitly defer:** proprietary CDN, proprietary global DRM, full production asset editing, complex ad exchange, broad TV/console coverage, microservices per module, multi-region active-active and advanced AI scene understanding before core distribution and analytics work reliably.

## 7. Functional requirements

Priority notation: **P0** = release blocker, **P1** = important next
release, **P2** = later enhancement.

### 7.1 Identity, accounts and profiles

The README specifies registration, verification, login/logout, refresh
tokens, OAuth, MFA, session management, multiple profiles and account
deletion.

-   **AUTH-01 (P0):** A user can register, verify email, sign in and
    sign out.
-   **AUTH-02 (P0):** A user can list and revoke sessions, including
    sign out of all sessions.
-   **AUTH-03 (P0):** Sensitive actions require recent authentication or
    MFA where policy requires it.
-   **AUTH-04 (P1):** An account can contain multiple viewing profiles
    with separate preferences and watch histories.
-   **AUTH-05 (P0):** Account deletion initiates a documented data
    deletion/retention workflow, subject to legal obligations.
-   **AUTH-06 (P0):** Device registration is explicit and revocable;
    offline-download device limits are enforced server-side.

### 7.2 Studio workspace, title operations and distribution

- **FR-STUDIO-001 (P0):** A verified studio account can create a public profile with name, description, logo, contact and brand assets.
- **FR-STUDIO-002 (P0):** Studio owners can invite collaborators and assign scoped roles (owner, admin, editor, analyst, screener manager); every privileged action is auditable.
- **FR-STUDIO-003 (P0):** A studio can create a title workspace linked to one canonical catalog title, with release status, ownership/rights evidence references and required deliverables.
- **FR-STUDIO-004 (P0):** Title readiness is explicit: metadata, poster/artwork, master, trailer, subtitles/audio tracks, classification, rights, pricing/offer and moderation status.
- **FR-STUDIO-005 (P0):** A studio can publish a title to its ViewAura storefront and the ViewAura marketplace without duplicating title identity or media assets.
- **FR-STUDIO-006 (P1):** A studio can schedule publication and unpublication using a server-side release schedule and timezone-aware display.
- **FR-STUDIO-007 (P1):** A studio can create a distribution offer with access type, price/currency, territory, start/end time, eligible channel, viewer limits and applicable terms.
- **FR-STUDIO-008 (P1):** A title may have multiple concurrent offers only when rights policy permits; entitlement evaluation must resolve the applicable offer deterministically.
- **FR-STUDIO-009 (P1):** A studio can generate shareable campaign links and promo codes with usage limits and attribution metadata.
- **FR-STUDIO-010 (P1):** A studio can view title-level processing, publication, playback, conversion, revenue and cost summaries.
- **FR-STUDIO-011 (P2):** A studio can configure an embeddable player and external distribution page while authorization remains governed by ViewAura rights/entitlements.
- **FR-STUDIO-012 (P2):** A studio can distribute through private event, festival or institution channels with bounded audience, schedule and reporting.

### 7.3 Rights, offers and channel policy

- **FR-RIGHTS-001 (P0):** Rights are modeled separately from catalog metadata and payment status.
- **FR-RIGHTS-002 (P0):** Rights records identify title/asset, rights holder, territory, permitted use, distribution channel, start/end window, download permission and evidence/reference.
- **FR-RIGHTS-003 (P0):** Publication and playback are blocked when the required rights data is absent, expired, conflicting or under review.
- **FR-RIGHTS-004 (P1):** Rights changes trigger reevaluation of active offers, playback grants, screener links and download licenses.
- **FR-RIGHTS-005 (P1):** Operators can audit who changed rights, when, why and against which evidence.

### 7.4 Audience relationships, campaigns and portability

- **FR-AUD-001 (P0):** Viewers can follow a studio and optionally a filmmaker; follow state is profile-scoped and privacy-controlled.
- **FR-AUD-002 (P1):** Studios can publish release updates to followers through in-app notifications and supported outbound channels only where the viewer has opted in.
- **FR-AUD-003 (P1):** Notification preferences support channel/topic controls, unsubscribe and suppression enforcement.
- **FR-AUD-004 (P1):** Campaign links record campaign/source identifiers and connect visits, offer views, checkout and successful transactions without exposing unnecessary personal data.
- **FR-AUD-005 (P1):** Studio analytics distinguish aggregate anonymous metrics from identifiable audience records; access to identifiable data is restricted and logged.
- **FR-AUD-006 (P1):** Studios can export permitted audience relationship and transaction records in a documented format, subject to user consent, legal basis, retention rules and platform policy.
- **FR-AUD-007 (P2):** Studios can create audience segments using consented first-party signals (e.g. followers, purchasers, event attendees) with minimum cohort thresholds and privacy safeguards.
- **FR-AUD-008 (P2):** Studios can run release campaigns and compare campaign conversion without unrestricted cross-site tracking.

### 7.5 Catalog and discovery

-   **CAT-01 (P0):** Users can browse and search movies, people, genres
    and credits.
-   **CAT-02 (P0):** Movie pages distinguish ViewAura playback,
    third-party streaming links and unavailable titles.
-   **CAT-03 (P0):** Metadata sources and last-enriched timestamps are
    retained for provenance and troubleshooting.
-   **CAT-04 (P1):** Users can filter by genre, language, year, runtime,
    availability, subtitle and audio options.
-   **CAT-05 (P1):** Catalog pages expose content advisories, maturity
    rating, captions and accessibility metadata when known.
-   **CAT-06 (P1):** Search and discovery have cached, useful fallback
    results when the search service is unavailable.

### 7.6 Ratings, reviews, lists and social

Preserve the README's rating, review, reaction, spoiler, watchlist,
custom-list, collaboration, follow, feed, discussion and challenge
capabilities.

-   **SOC-01 (P0):** A profile can create or update one rating per
    movie.
-   **SOC-02 (P0):** A profile can create, edit and delete reviews,
    including spoiler marking.
-   **SOC-03 (P0):** Watchlist and list changes are profile-scoped and
    idempotent.
-   **SOC-04 (P1):** Users can follow profiles and view an activity
    feed.
-   **SOC-05 (P1):** Users can create collaborative lists and discussion
    threads with permission controls.
-   **SOC-06 (P1):** Private lists and activity can be excluded from
    public feeds and recommendations according to user settings.

### 7.7 Creator upload and publishing

-   **UP-01 (P0):** Creators can initiate direct-to-object-storage
    uploads using short-lived signed upload credentials.
-   **UP-02 (P0):** Uploads support chunking/resumption where supported
    by the selected storage provider.
-   **UP-03 (P0):** The API reserves storage/processing quota at
    initiation and releases it on abort or terminal failure.
-   **UP-04 (P0):** Upload completion triggers a durable workflow;
    duplicate completion events do not create duplicate processing jobs.
-   **UP-05 (P0):** The platform validates file type using content
    inspection, not filename alone; enforces size, duration, codec and
    resolution limits.
-   **UP-06 (P0):** Raw originals are private and quarantined until
    validation and moderation pass.
-   **UP-07 (P0):** Creators can see upload, transcode, moderation and
    publish states with actionable error messages.
-   **UP-08 (P1):** Creators can upload poster art, captions, subtitle
    files, audio tracks and metadata.
-   **UP-09 (P1):** Creators can manage versions, replace assets and
    roll back publication safely.
-   **UP-10 (P1):** A published title can be scheduled, geo-restricted,
    withdrawn or placed back into review.

### 7.8 Streaming and playback

-   **STR-01 (P0):** The player obtains a playback session from the
    backend before accessing protected media.
-   **STR-02 (P0):** Playback authorization checks account/profile
    status, title rights, territory, subscription/PPV entitlement,
    device policy and time window.
-   **STR-03 (P0):** Video is delivered as adaptive HLS with multiple
    renditions appropriate to source quality and target devices.
-   **STR-04 (P0):** Manifests and segments are delivered through a CDN;
    private origins are not publicly enumerable.
-   **STR-05 (P0):** Playback credentials are short-lived, scoped to an
    asset/session and revocable. Do not place long-lived secrets in
    client code or URLs.
-   **STR-06 (P0):** The player supports play/pause, seek, resume, auto
    quality and a manual quality override where the platform supports
    it.
-   **STR-07 (P0):** Playback progress is periodically checkpointed and
    finalized on pause/exit/completion when connectivity allows.
-   **STR-08 (P1):** The player supports multiple subtitle tracks and
    audio tracks where available.
-   **STR-09 (P1):** The player exposes a data-saver mode, including a
    maximum quality preference.
-   **STR-10 (P1):** The player handles expired credentials, temporary
    CDN failures and network changes without silently losing the viewing
    position.
-   **STR-11 (P1):** Watch-party playback uses a room-authorized session
    and synchronizes play/pause/seek state; the server is authoritative
    for room state.
-   **STR-12 (P1):** Playback events include startup time, rebuffering,
    bitrate, resolution, errors, CDN/ISP region and session end reason,
    with privacy-aware retention.

### 7.9 Offline mode and intermittent connectivity

Offline behavior must be designed as a supported mode, not simply as
caching everything in a web browser.

-   **OFF-01 (P0):** The app clearly indicates online, offline and
    synchronization states.
-   **OFF-02 (P0):** The client can retain a bounded offline catalog
    cache for recently viewed/saved titles, including essential
    metadata, artwork and availability state.
-   **OFF-03 (P0):** Users can access cached watchlists and selected
    lists without a connection.
-   **OFF-04 (P0):** The client can queue supported mutations
    (e.g. watch progress, rating, review draft, list edits) in a durable
    local outbox.
-   **OFF-05 (P0):** Each queued mutation has a unique operation ID,
    profile ID, entity ID, local timestamp, schema version and
    idempotency key.
-   **OFF-06 (P0):** Synchronization retries safely and never assumes a
    request failed merely because the response was lost.
-   **OFF-07 (P0):** Conflicts are handled by explicit domain rules; the
    client must not silently overwrite a newer server value.
-   **OFF-08 (P0):** Offline playback is available only for titles with
    an explicit download entitlement and supported client.
-   **OFF-09 (P0):** Offline files are encrypted at rest, bound to an
    authorized app/device and protected by an expiring license where
    rights require it.
-   **OFF-10 (P0):** Expired, revoked or invalid offline licenses
    prevent further playback; the app explains the reason and next
    required action.
-   **OFF-11 (P1):** Users can choose Wi-Fi-only downloads, download
    quality, storage limits and automatic cleanup.
-   **OFF-12 (P1):** Downloads can pause, resume, retry and be deleted;
    the app reports storage usage and failure cause.
-   **OFF-13 (P1):** The app supports a configurable offline retention
    window and periodic license renewal while online.
-   **OFF-14 (P1):** Offline playback progress is merged with server
    progress on reconnection without duplicate completion events.
-   **OFF-15 (P2):** Optional background prefetch downloads upcoming
    episodes or user-selected titles only when device/network policy
    permits.

**Platform constraint:** The README explicitly says native mobile apps
are not part of the current repository. Reliable encrypted offline video
downloads generally require native client capabilities and
platform-specific secure storage/DRM support. A browser/PWA can cache
metadata and some app shell resources, but must not be represented as
equivalent to a protected native offline-video implementation. The
client platform decision is therefore a release dependency.

### 7.10 Vaulting and private media

"Vault" is defined as three related but distinct product capabilities.
They must not be conflated.

#### A. Creator asset vault

-   **VLT-01 (P0):** Every uploaded original is private by default and
    associated with an owner, organization/workspace and asset record.
-   **VLT-02 (P0):** Asset states include `initiated`, `uploading`,
    `quarantined`, `processing`, `approved`, `published`, `rejected`,
    `failed`, `archived` and `deleted`.
-   **VLT-03 (P0):** Vault assets support metadata, checksums, file
    versions, created/updated timestamps and retention policy.
-   **VLT-04 (P0):** Access is checked on every operation using
    owner/workspace roles and resource-level permissions.
-   **VLT-05 (P0):** Raw originals, mezzanine files and packaged
    playback renditions have distinct storage classes and access
    policies.
-   **VLT-06 (P1):** Creators can organize assets into
    folders/collections, add labels and search their assets.
-   **VLT-07 (P1):** Deletion is a lifecycle workflow: revoke access,
    mark pending deletion, apply retention/legal hold, then purge
    objects and derivatives.

#### B. Screener vault

-   **VLT-08 (P0 for screener release):** A screener is accessible only
    through an explicit invitation or license.
-   **VLT-09:** Access policy can specify recipient, email/account,
    device count, territory, start/end time, playback count and download
    permission.
-   **VLT-10:** Screeners are not publicly discoverable and are excluded
    from public search/recommendations.
-   **VLT-11:** Watermarking can identify a recipient/session without
    exposing another recipient's information.
-   **VLT-12:** Revocation prevents new playback sessions and is
    enforced at license/session renewal boundaries.
-   **VLT-13:** Audit records capture invitation, access grant, playback
    start/end, failed access and revocation.

#### C. Viewer offline vault

-   **VLT-14 (P0 for offline release):** The viewer can see eligible
    downloads, license expiry, device binding, file size and download
    state.
-   **VLT-15:** Downloaded content is stored in app-controlled encrypted
    storage, not a user-browsable public folder.
-   **VLT-16:** App reinstall, device replacement, logout and account
    deletion follow explicit key/license invalidation rules.
-   **VLT-17:** A user can remove a download without cancelling the
    underlying purchase/subscription.
-   **VLT-18:** Offline access follows the title's distribution rights
    and license, not merely the user's subscription status.

**Not included:** A "vault" must not imply that ViewAura can defeat DRM,
copy protection or rights restrictions. The product must not offer
unrestricted downloads of third-party licensed content.

### 7.11 Payments, subscriptions and entitlements

The README specifies Stripe, subscriptions, invoices, PPV, screener
licenses, per-movie checks and tier-based upload/storage quotas.

-   **PAY-01 (P0):** Entitlements are represented independently from
    payment provider events.
-   **PAY-02 (P0):** Webhook processing is signature-verified,
    idempotent and replay-safe.
-   **PAY-03 (P0):** Playback and download authorization use a
    centralized entitlement policy.
-   **PAY-04 (P0):** Subscription cancellation, failed payment, refund,
    chargeback and expiry update entitlement state consistently.
-   **PAY-05 (P0):** A download license is a separate grant with its own
    expiry, device and territory constraints.
-   **PAY-06 (P1):** The user can view active plans, purchases, invoices
    and device/download limits.
-   **PAY-07 (P1):** Creators can see quota usage and upgrade paths
    before an upload fails.

### 7.12 Moderation, rights and trust

-   **MOD-01 (P0):** User reports and automated screening can create
    moderation cases.
-   **MOD-02 (P0):** Unreviewed or quarantined media is never publicly
    playable.
-   **MOD-03 (P0):** Moderators can lock cases, decide, escalate and
    process appeals.
-   **MOD-04 (P0):** Every published asset has a recorded
    rights/ownership declaration and permitted territories/windows.
-   **MOD-05 (P1):** Rights expiry can automatically unpublish or block
    playback and downloads.
-   **MOD-06 (P1):** Moderation decisions and rights changes are
    auditable.

### 7.13 Notifications

-   **NOT-01 (P1):** Users can receive in-app notifications for upload
    status, download/license status, social activity and account
    security.
-   **NOT-02 (P1):** Notification preferences are configurable by type
    and channel.
-   **NOT-03 (P0):** Security and payment notices cannot be silently
    disabled where legally or operationally required.

## 8. Offline synchronization rules

Offline mutations are not all equal. Apply domain-specific conflict
policies.

  -----------------------------------------------------------------------
  Data/action                         Suggested policy
  ----------------------------------- -----------------------------------
  Watch progress                      Merge by most recent valid
                                      checkpoint; preserve completion
                                      once server-confirmed

  Rating                              Last-write-wins using server-issued
                                      version and timestamp; optionally
                                      prompt if concurrent edits are
                                      detected

  Review draft                        Append/sync draft versions; never
                                      silently replace a newer server
                                      revision

  Published review edit               Optimistic concurrency with
                                      version/ETag; conflict requires
                                      user resolution

  Watchlist membership                Idempotent set operation (`add` /
                                      `remove`), not blind toggle

  Custom list ordering                Versioned reorder operation; server
                                      resolves or asks user when
                                      concurrent reorder conflicts

  Profile/security/payment changes    Online-only; never queue sensitive
                                      account mutations

  Offline license                     Server-issued, time-bounded
                                      entitlement; not created by client
  -----------------------------------------------------------------------

**Sync contract:** The server acknowledges each operation ID and returns
`accepted`, `already_applied`, `conflict`, `rejected` or
`requires_online`. The client retains unresolved operations and exposes
them to the user. The server is authoritative for final state.

## 9. Non-functional requirements and service objectives

Targets below are proposed acceptance targets, not verified current
performance. The README's original targets include movie-page p95 under
50 ms, watch-party sync under 100 ms, 100K+ system-wide RPS and 99.99%
availability. Those are ambitious platform-wide aspirations and should
not be used as initial MVP acceptance gates without load tests and a
funded operating model.

  -----------------------------------------------------------------------
  Area                    Initial target          Notes
  ----------------------- ----------------------- -----------------------
  Catalog page            p95 ≤ 300 ms API time   Measure separately from
                          for uncached origin;    media startup
                          edge cached reads       
                          faster                  

  Search                  p95 ≤ 300 ms for normal Avoid claiming 1 ms
                          queries                 end-to-end

  Playback startup        p95 ≤ 3 seconds on      Measure by region, ISP
                          defined test            and rendition
                          network/device          

  Rebuffering             \< 1% of playback time  Segment by
                          on supported network    device/network
                          test profiles           

  Playback availability   ≥ 99.9% monthly for MVP Raise only with
                          streaming path          evidence and budget

  API availability        ≥ 99.9% monthly for MVP Exclude planned
                                                  maintenance only by
                                                  policy

  Upload resumability     No need to restart      Test process and
                          completed chunks after  network interruptions
                          transient failure       

  Offline sync            No lost acknowledged    Test crash/restart and
                          operation; duplicate    replay
                          requests are idempotent 

  Security                No public raw media;    Threat-model before
                          short-lived playback    launch
                          grants; auditable       
                          privileged access       

  Accessibility           Captions support and    Expand to audio
                          keyboard/player         description as catalog
                          accessibility on        allows
                          supported clients       

  Data efficiency         User-selected max       Measure MB per playback
                          quality and Wi-Fi-only  minute
                          download controls       

  Observability           Correlated request,     Avoid logging secrets
                          upload, workflow and    or raw tokens
                          playback identifiers    
  -----------------------------------------------------------------------

## 10. User experience requirements

### 10.1 Playback experience

The title page must show: - Whether ViewAura can play the title or only
link to an external provider. - Price/plan requirement and any territory
restriction. - Available video quality, audio and subtitle options where
known. - Download eligibility and license conditions. - A clear "Play"
or "Watch trailer" action; never imply full playback availability when
only a trailer or external link exists.

The player must: - Remember progress. - Recover from transient network
errors. - Show a clear network/quality state. - Offer data saver and
manual quality selection. - Preserve the last known position after an
app restart. - Distinguish a rights/entitlement failure from a network
failure.

### 10.2 Offline experience

The app should show a clear offline banner without blocking cached
content. A download detail should include progress, size, quality,
remaining license period, retry and delete controls. Queued social
actions should show "Pending sync" until acknowledged by the server.

### 10.3 Vault experience

Creator asset vault screens should distinguish originals, processing
outputs and published playback assets. Private screeners should show
recipient and expiry controls. Viewer downloads should be presented as a
personal offline library, not as ownership of an unrestricted media
file.

## 11. Data and domain model requirements

The README already defines users, profiles, movies, media assets,
subscriptions, invoices, screener licenses, moderation cases and
engagement tables. Add or formalize the following entities as needed:

-   `media_assets`: owner, workspace, asset type, lifecycle status,
    storage key, checksum, size, duration, source metadata, retention
    class.
-   `media_asset_versions`: asset ID, version, object key, checksum,
    created by, status.
-   `media_renditions`: asset ID, codec, container, resolution, bitrate,
    audio profile, manifest key, processing status.
-   `playback_sessions`: profile, asset/movie, entitlement reference,
    device, token/session state, started/last-seen/ended timestamps.
-   `playback_progress`: profile, movie, position, duration, completion
    state, version.
-   `download_grants`: profile, asset/movie, device, license ID,
    granted/expiry/revocation timestamps, territory, policy.
-   `device_registrations`: account/profile, device public identifier,
    platform, last seen, revoked state.
-   `offline_sync_operations`: operation ID, profile, operation type,
    entity ID, payload version, status, server result.
-   `vault_access_audit`: actor, asset, action, decision, timestamp,
    request/device context.
-   `content_rights`: owner/licensor, title/asset, territories, allowed
    use, start/end, download/offline flags.
-   `subtitle_tracks` and `audio_tracks`: language, label, format,
    asset/rendition reference, accessibility flags.

These are logical requirements, not a mandate to add every table
immediately. Keep module ownership clear, avoid cross-module table
reads, and use events for downstream consumers.

## 12. Analytics and success metrics

Track the README's discovery, recommendation, watch-completion, upload,
moderation and QoE metrics, and add:

### Discovery

-   Search-to-title click-through.
-   Recommendation click-through and subsequent watch starts.
-   Catalog result zero-result rate.

### Playback

-   Playback start success.
-   Startup time p50/p95/p99.
-   Rebuffer ratio and rebuffer count.
-   Fatal playback errors by platform, codec, CDN, ISP and region.
-   Average delivered bitrate and data consumed per hour.
-   Completion rate, abandonment and resume success.

### Offline

-   Download start/success/failure.
-   Download retry and resume success.
-   Offline playback success.
-   License renewal and expiry failure.
-   Sync queue age, conflict rate and rejected operation rate.
-   Storage used per user/device.

### Creator and vault

-   Upload completion and processing success.
-   Upload-to-publish duration.
-   Transcode cost per source hour and per delivered viewing hour.
-   Storage by original/rendition/archive class.
-   Vault access denials, suspicious access and deletion completion
    time.

### Studio distribution and audience

- Time from studio signup to first published title.
- Time from publication to first paid transaction and first payout.
- Active publishing studios and repeat publishing rate.
- Titles published per studio and percentage of titles with complete rights/deliverables.
- Studio gross revenue, net revenue, fees, refunds and payout success.
- Offer impressions → title-page visits → playback starts → paid conversions.
- Campaign-attributed visits and transactions.
- Followers per studio/title, notification opt-in and unsubscribe rates.
- Repeat viewers per studio and title-level completion/revisit rate.
- Studio support burden and manual operations per published title.
- Cost per source-hour processed, GB stored, viewing hour delivered and paid transaction.

### Business

-   Paid conversion, subscription retention and PPV conversion.
-   Revenue per viewing hour.
-   CDN egress and storage cost per active viewer.
-   Gross margin per title/cohort where data is available.

## 13. Privacy, safety and compliance requirements

-   Collect only telemetry needed for product reliability, security,
    billing and agreed analytics.
-   Avoid sending email, passwords, raw JWTs, signed URLs or license
    secrets to analytics.
-   Define retention and deletion policies for playback events, vault
    audit records, uploads and user content.
-   Provide account/profile privacy controls and consent where required.
-   Confirm territorial licensing, age classification, copyright
    takedown and data-protection obligations with qualified counsel for
    launch markets.
-   Do not use automated moderation as the only appeal path for
    consequential decisions.
-   Secure screener watermarking and audit logs as sensitive data.
-   Treat downloaded media and license keys as protected assets.

## 14. Acceptance criteria for launch

### Studio distribution MVP

- [ ] A studio owner can create a studio workspace and invite a collaborator with a scoped role.
- [ ] A studio can create a title workspace and upload required media directly to private object storage.
- [ ] The system blocks publication when required rights, moderation or title-readiness checks fail.
- [ ] A title can be published to a studio page and ViewAura marketplace without duplicating the canonical title or asset.
- [ ] A distribution offer is constrained by territory, channel, use and validity window.
- [ ] A viewer can acquire an eligible offer and receive a normalized entitlement.
- [ ] A completed transaction is recorded in the revenue ledger and can be reconciled with the payment provider.
- [ ] Studio analytics show basic views, playback starts, completion, paid conversion, gross/net revenue and delivery cost.
- [ ] Studio roles, rights changes, publication actions and screener access are auditable.

### Audience relationships

- [ ] A viewer can follow/unfollow a studio and control visibility of that relationship.
- [ ] Release notifications are sent only according to consent and preference settings.
- [ ] Campaign attribution connects campaign links to eligible offer views and transactions without leaking sensitive personal data.
- [ ] Audience exports are permission-checked, time-limited, audited and limited to legally permitted fields.

### Streaming MVP

-   [ ] An approved asset can be transcoded into a tested HLS rendition
    ladder.
-   [ ] A viewer without entitlement cannot retrieve a playable manifest
    or segment.
-   [ ] A viewer with entitlement receives a short-lived playback
    session.
-   [ ] Playback adapts to bandwidth changes and resumes from saved
    progress.
-   [ ] Raw source objects are not public.
-   [ ] Playback QoE events can be traced to a session without exposing
    credentials.
-   [ ] CDN cache behavior does not leak private content between users.

### Offline MVP

-   [ ] Supported client can download an eligible title.
-   [ ] Download is encrypted and not exposed as an ordinary media file.
-   [ ] Playback works in airplane mode within license policy.
-   [ ] Expired/revoked license is enforced.
-   [ ] Download can pause/resume and recover after app restart.
-   [ ] Offline progress synchronizes once connectivity returns.
-   [ ] Duplicate sync submissions do not duplicate server-side actions.

### Vault MVP

-   [ ] New uploads are private and quarantined.
-   [ ] Access is denied to non-owners/non-authorized collaborators.
-   [ ] Asset lifecycle and processing outputs are visible to authorized
    creators.
-   [ ] Revocation blocks new access.
-   [ ] Audit records exist for sensitive vault actions.
-   [ ] Deletion follows retention and legal-hold rules.

### Operational readiness

-   [ ] Upload and playback dashboards and alerts exist.
-   [ ] Backup/restore and object lifecycle policies are tested.
-   [ ] Secrets are in a secret manager, not source control.
-   [ ] Load, failure-injection and security tests pass agreed release
    thresholds.
-   [ ] Runbooks exist for failed transcodes, CDN/origin outage,
    entitlement outage, key/license issues and data restore.

## 15. Assumptions, open decisions and risks

  -----------------------------------------------------------------------
  Topic                               Current position / decision needed
  ----------------------------------- -----------------------------------
  Client platforms                    README says native apps are out of
                                      current repo. Choose web first,
                                      native Android/iOS, or both;
                                      protected offline video requires
                                      native implementation or a suitable
                                      DRM-capable client.

  Streaming provider                  README names R2 / Cloudflare Stream
                                      / CDN. Decide managed Stream versus
                                      R2 + self-managed packaging after a
                                      cost and feature comparison.

  DRM                                 Explicitly not implemented in
                                      README. Determine rights-holder
                                      requirements before selecting
                                      Widevine/FairPlay/PlayReady or
                                      other DRM.

  Offline rights                      Not every title can be downloaded.
                                      Contractual rights must specify
                                      offline permission and expiry.

  Vault definition                    This PRD separates creator
                                      originals, private screeners and
                                      viewer offline downloads. Confirm
                                      whether the product also intends
                                      personal video uploads or private
                                      user collections.

  Launch geography                    Define launch territories, payment
                                      availability, content rights,
                                      data-protection and CDN performance
                                      requirements.

  Scale target                        Validate the README's 100K+ RPS and
                                      99.99% goals against real demand
                                      and operating budget.

  Recommendations                     Start with reliable content-based
                                      and behavioral ranking; advanced
                                      embedding infrastructure can follow
                                      usage.

  Analytics pipeline                  README lists ClickHouse and
                                      Flink-style jobs; Flink jobs are
                                      not present. Start with
                                      batch/consumer aggregation if
                                      sufficient.
  -----------------------------------------------------------------------

## 16. Recommended delivery order

1. **Validate studio demand:** five pilot studios, one title each, workflow and willingness-to-pay evidence.
2. **Security and domain correctness:** studio tenancy/roles, rights, offers, entitlements, asset state machine, quotas and audit.
3. **Studio publishing and first revenue:** upload → validate → package → moderate → rights gate → publish → offer → checkout → payout.
4. **Distribution and measurement:** branded page, playback, campaign attribution, studio analytics and unit economics.
5. **Audience relationships:** follows, opt-in notifications, repeat-viewer insights and permitted exports.
6. **Vault and screeners:** versioning, roles, watermarking, expiry, retention and audit.
7. **Offline and resilience:** metadata sync first, then protected downloads for licensed content.
8. **Scale selectively:** only split services or add multi-region/DRM complexity after evidence.

## 17. Traceability to the supplied README

This PRD preserves the README's stated product framing, personas,
modular Go core, Kafka event backbone, Temporal workflows, Rust/Python
satellite services, Cloudflare edge,
PostgreSQL/Redis/MeiliSearch/ClickHouse/R2 data architecture, Stripe
monetization, moderation and analytics goals.

The README's existing functional requirements are retained and expanded.
The following are explicitly **new or materially expanded requirements**
in this document: adaptive playback sessions and QoE, offline-first
metadata and mutation synchronization, protected offline downloads, the
three-part vault model, rights-aware download licenses, device
registration, content-rights windows, and related acceptance criteria.

The README's roadmap and known gaps remain authoritative for
implementation status. A requirement in this PRD does not mean its
implementation already exists.
