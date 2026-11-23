# ViewAura Product Requirements Document — Non-MVP Product

**Document status:** Proposed post-MVP product specification  
**Product:** ViewAura  
**Version:** 1.0 — Non-MVP scope  
**Date:** 23 November 2026  
**Relationship to MVP:** This document specifies the capabilities that follow the initial studio-distribution MVP. It is not a replacement for the MVP PRD. MVP foundations such as identity, studio workspaces, title publishing, rights checks, basic offers, checkout, HLS playback and basic studio reporting are assumed to exist and be operational.

---

## 1. Purpose and product direction

ViewAura's post-MVP product evolves from a reliable publishing and monetization workflow into a full operating and distribution platform for independent media businesses.

The product must deepen the advantage that a small media house gains from using ViewAura: it should manage its catalogue, media assets, rights, release strategy, distribution channels, audience relationships, campaigns, screeners, institutional customers and commercial performance from one connected system.

The product is not intended to reproduce the catalogue scale or infrastructure footprint of major streaming corporations. Its defensibility comes from film-specific operational workflows, creator-audience relationships, multi-channel distribution, rights-aware commerce, media security and commercial intelligence.

### 1.1 Product surfaces

- **ViewAura Studio:** Full studio operations, catalogue and asset management, rights, release planning, audience CRM, campaigns, revenue, collaborators and analytics.
- **ViewAura Watch:** Consumer discovery, streaming, communities, social film activity, personalization and resilient access.
- **ViewAura Distribution:** Embeddable playback, branded external pages, private screenings, event/festival/institution delivery, partner integrations and distribution APIs.

These are product surfaces over shared platform domains, not necessarily separate services.

## 2. Post-MVP objectives

1. Let studios operate a catalogue and release slate, not merely upload individual films.
2. Make one title distributable across multiple authorized channels without duplicating assets or rights logic.
3. Give rights holders a robust media vault for masters, versions, subtitles, artwork, screeners and delivery packages.
4. Support controlled access for reviewers, buyers, festivals, institutions and corporate customers.
5. Help studios build permissioned, portable relationships with their audiences.
6. Connect marketing campaigns to viewing, conversion, repeat engagement and revenue.
7. Expand monetization beyond a single rental or subscription model.
8. Deliver reliable experiences in bandwidth-constrained environments, including licensed offline access.
9. Introduce operational intelligence that improves title economics and release decisions.
10. Scale infrastructure only when usage, contractual obligations or unit economics justify it.

## 3. Scope boundaries

This document covers post-MVP capabilities. It assumes the MVP has already delivered the core studio-to-first-revenue path.

### Included post-MVP capabilities

- Advanced media vault, versioning, retention and lifecycle policies.
- Forensic/visible watermarking and enhanced screener controls.
- Branded external studio pages and embeddable players.
- Advanced release calendar, territory/window management and campaign tooling.
- Audience segments, permitted exports, consent management and CRM-like workflows.
- Advanced analytics, attribution, revenue reconciliation and title-level profitability.
- Bundles, memberships, early access, paid premieres and institution/event licensing.
- Festival, campus, community cinema and corporate screening workflows.
- Licensed offline downloads, smart preloading, download scheduling and data controls.
- Localization, subtitle/audio workflows and accessibility metadata.
- Advanced discovery, cross-studio recommendations and curated collections.
- Partner distribution APIs and channel adapters.
- Optional licensed local caching/community distribution, subject to legal and economic validation.

### Explicitly not implied

- ViewAura becoming a global CDN or DRM vendor.
- Unlimited downloads or perpetual access independent of rights.
- Unrestricted export or sale of viewer personal data.
- A full professional post-production editing suite.
- Guaranteed availability on every TV, console or mobile platform.
- Multi-region active-active writes by default.
- Advanced AI features without a validated user problem and measurable return.

## 4. Functional requirements

Priority labels: **P1** = important post-MVP capability; **P2** = advanced capability; **P3** = strategic/conditional capability.

### 4.1 Studio operations and catalogue management

- **NM-STU-001 (P1):** A studio can manage multiple titles and releases through a slate/calendar view.
- **NM-STU-002 (P1):** A title workspace supports multiple deliverable versions, including original master, distribution master, trailer, teaser, artwork, subtitles, captions and audio tracks.
- **NM-STU-003 (P1):** A studio can assign collaborators to title-level workspaces with scoped roles and expiry.
- **NM-STU-004 (P1):** A studio can configure approval steps for metadata, media, rights, pricing and publication.
- **NM-STU-005 (P2):** A studio can manage collections, franchises, seasons, episodic content and related title groupings.
- **NM-STU-006 (P2):** Studio teams can create reusable release templates and metadata presets.
- **NM-STU-007 (P2):** Studio activity history provides a searchable audit of changes to assets, rights, offers, collaborators and publication.
- **NM-STU-008 (P2):** Studio administrators can export their operational records and supported asset manifests in documented formats.

### 4.2 Media vault and asset lifecycle

- **NM-VLT-001 (P1):** Assets are versioned; a new version never silently overwrites an existing approved master.
- **NM-VLT-002 (P1):** Each asset records owner, title, asset type, checksum, size, codec/container metadata, language, version, creation time, retention class and processing state.
- **NM-VLT-003 (P1):** Studio users can compare versions and identify which distribution channels use each version.
- **NM-VLT-004 (P1):** Assets can be tagged and organized into collections, folders or production/delivery workspaces.
- **NM-VLT-005 (P1):** Retention policies support legal hold, archival, scheduled deletion and recovery windows.
- **NM-VLT-006 (P1):** Asset deletion checks dependencies such as active offers, screeners, playback packages and contractual retention.
- **NM-VLT-007 (P1):** Every sensitive asset action is audited, including view, download, share, permission change, replacement and deletion.
- **NM-VLT-008 (P2):** A studio can create expiring, password-protected or recipient-bound asset shares where policy permits.
- **NM-VLT-009 (P2):** The vault supports integrity verification and periodic checksum audits.
- **NM-VLT-010 (P2):** Lifecycle policies can move cold originals to lower-cost archival storage while preserving metadata and restore workflows.

### 4.3 Screener and review-room management

- **NM-SCR-001 (P1):** A studio can create named screener campaigns for reviewers, buyers, press, juries or partners.
- **NM-SCR-002 (P1):** Access can be bound to an invited identity, email verification, device limit, time window and permitted territory.
- **NM-SCR-003 (P1):** Watermarking can include visible recipient identifiers and forensic marks where technically and contractually appropriate.
- **NM-SCR-004 (P1):** Studio administrators can revoke access immediately and inspect access history.
- **NM-SCR-005 (P1):** Download, playback, casting and screen-capture restrictions are explicit policy controls; the product must not promise that client-side controls prevent all copying.
- **NM-SCR-006 (P2):** Review rooms support notes, time-coded comments, reviewer forms and structured feedback.
- **NM-SCR-007 (P2):** Review-room participants can be grouped into campaigns with separate access windows and reporting.
- **NM-SCR-008 (P2):** Screener reports summarize invitation, access, viewing progress and feedback without exposing unrelated viewer data.

### 4.4 Distribution channels and branded experiences

- **NM-DST-001 (P1):** A studio can configure an embeddable player with branding, allowed domains, playback options and access policy.
- **NM-DST-002 (P1):** An embed must use the same rights, offer, entitlement and playback authorization as ViewAura Watch.
- **NM-DST-003 (P1):** A studio can publish a branded external storefront or microsite using its ViewAura catalogue and offers.
- **NM-DST-004 (P1):** External pages support campaign attribution and conversion reporting.
- **NM-DST-005 (P2):** A distribution adapter can publish or provide access through an approved partner channel.
- **NM-DST-006 (P2):** Channel configuration identifies territories, windows, format requirements, commercial terms and revocation behavior.
- **NM-DST-007 (P2):** A title can be offered through multiple channels concurrently only where rights and exclusivity constraints permit.
- **NM-DST-008 (P3):** Partner APIs support catalogue, availability, offer and playback-session operations with scoped credentials and usage quotas.

### 4.5 Rights, licensing and release management

- **NM-RGT-001 (P1):** Rights records support multiple rights holders, territories, media formats, permitted uses and distribution channels.
- **NM-RGT-002 (P1):** Rights windows can be exclusive or non-exclusive and can include holdbacks, embargoes and blackout periods.
- **NM-RGT-003 (P1):** A rights dashboard highlights upcoming expirations, conflicts, missing evidence and offers affected by a rights change.
- **NM-RGT-004 (P1):** Rights changes trigger re-evaluation of offers, embeds, screener grants, offline licenses and institutional access.
- **NM-RGT-005 (P2):** Studios can attach contracts or evidence references with access restrictions and version history.
- **NM-RGT-006 (P2):** The platform can model revenue-share terms and reporting obligations by title, channel, territory or partner.
- **NM-RGT-007 (P2):** Release calendar supports staggered release windows across territories and channels.
- **NM-RGT-008 (P3):** Contract metadata extraction may assist data entry, but human approval remains authoritative for rights decisions.

### 4.6 Monetization and commerce

- **NM-COM-001 (P1):** Support bundles containing multiple titles with shared or title-specific access rules.
- **NM-COM-002 (P1):** Support paid premieres and scheduled event access.
- **NM-COM-003 (P1):** Support early-access windows for eligible memberships or purchaser groups.
- **NM-COM-004 (P2):** Support studio memberships or creator subscriptions with recurring benefits.
- **NM-COM-005 (P2):** Support institution-level licenses with seat, concurrent-viewer, territory and date limits.
- **NM-COM-006 (P2):** Support promo campaigns, gift codes and campaign-specific pricing subject to rights and tax configuration.
- **NM-COM-007 (P2):** Provide transaction ledger, refunds, chargebacks, payout status and reconciliation views.
- **NM-COM-008 (P2):** Show gross sales, fees, taxes, refunds, rights-holder shares and net payable amounts separately.
- **NM-COM-009 (P3):** Evaluate ad-supported access only when demand, rights, ad operations and privacy compliance justify it.

### 4.7 Audience relationships, CRM and portability

- **NM-AUD-001 (P1):** Viewers can follow studios, filmmakers, collections and selected titles with granular notification preferences.
- **NM-AUD-002 (P1):** Studios can create consent-based audience segments from followers, purchasers, event attendees and other permitted first-party signals.
- **NM-AUD-003 (P1):** Audience tools enforce suppression, unsubscribe, consent and lawful-basis rules across all outbound channels.
- **NM-AUD-004 (P1):** Campaign tools support scheduled announcements and release reminders.
- **NM-AUD-005 (P1):** Studios can export permitted audience and transaction records with provenance, consent status and export audit.
- **NM-AUD-006 (P2):** Studio CRM views show relationship history relevant to studio interactions without exposing private viewer activity unrelated to that studio.
- **NM-AUD-007 (P2):** Viewers can manage communication preferences and data portability through self-service controls.
- **NM-AUD-008 (P2):** Audience segments enforce minimum cohort thresholds for aggregate reporting and prevent sensitive inference.
- **NM-AUD-009 (P2):** Campaign attribution can connect approved external campaign identifiers to visits, offer views, purchases and subsequent viewing.
- **NM-AUD-010 (P3):** Studio-to-audience communication integrations may connect supported email/CRM providers through explicit authorization.

### 4.8 Viewer experience, discovery and community

- **NM-VWR-001 (P1):** Viewers can browse curated independent-film collections by region, language, theme, festival, filmmaker and studio.
- **NM-VWR-002 (P1):** Viewers can follow a studio or filmmaker and receive relevant release updates.
- **NM-VWR-003 (P1):** Watch history, progress, lists, reviews and preferences synchronize across supported devices.
- **NM-VWR-004 (P2):** Recommendation ranking can use explicit taste signals, viewing behavior and editorial curation with privacy controls.
- **NM-VWR-005 (P2):** Viewers can join moderated film clubs, watch parties and scheduled premieres.
- **NM-VWR-006 (P2):** Discovery can surface availability and price by territory and channel without implying rights where none exist.
- **NM-VWR-007 (P2):** Viewer-facing accessibility metadata includes captions, subtitles, audio descriptions and language availability when supplied.
- **NM-VWR-008 (P3):** Cross-studio discovery can recommend films from smaller catalogues without allowing paid placement to masquerade as organic ranking.

### 4.9 Offline and connectivity resilience

- **NM-OFF-001 (P1):** Supported native clients can download eligible titles only when rights and offer policy permit.
- **NM-OFF-002 (P1):** Offline packages are encrypted at rest and bound to a registered device and expiring license.
- **NM-OFF-003 (P1):** The app displays expiry, remaining offline validity, storage use and download eligibility before download.
- **NM-OFF-004 (P1):** Downloads resume after interruption and verify integrity before becoming playable.
- **NM-OFF-005 (P1):** Users can schedule downloads on Wi-Fi, select quality and set data/storage limits.
- **NM-OFF-006 (P2):** Smart preloading can cache upcoming episodes or user-selected titles under explicit storage and data budgets.
- **NM-OFF-007 (P2):** Offline watch progress and permitted interactions synchronize idempotently on reconnect.
- **NM-OFF-008 (P2):** The client resolves conflicts using server version, client sequence and explicit user intent; it must not silently overwrite newer state.
- **NM-OFF-009 (P2):** Network-aware playback adjusts rendition and retry behavior without creating excessive data use.
- **NM-OFF-010 (P3):** Licensed local caches for institutions or community venues may serve approved content to a bounded local audience, with rights, security, metering and audit controls.

### 4.10 Analytics and business intelligence

- **NM-ANL-001 (P1):** Studio dashboards report title/channel performance: impressions, offer views, starts, completion, paid conversion, refunds and net revenue.
- **NM-ANL-002 (P1):** Reports distinguish platform-distributed, embedded, private-screening and institutional activity.
- **NM-ANL-003 (P1):** Studio analytics expose processing, storage and delivery costs to support title-level unit economics.
- **NM-ANL-004 (P1):** Campaign attribution connects source, offer, transaction and aggregate viewing outcomes.
- **NM-ANL-005 (P2):** Cohort reporting compares first-time and repeat viewers, release windows and campaign performance.
- **NM-ANL-006 (P2):** Revenue reporting supports rights-holder shares, partner splits, fees, taxes and payout reconciliation.
- **NM-ANL-007 (P2):** Operational analytics track upload failures, processing latency, playback startup, rebuffering and CDN delivery cost.
- **NM-ANL-008 (P2):** Reports support CSV export and scheduled delivery with role-based access.
- **NM-ANL-009 (P3):** Forecasting may estimate title performance from historical data, but forecasts must show uncertainty and must not be represented as guaranteed revenue.

### 4.11 Localization and accessibility operations

- **NM-L10N-001 (P1):** Studios can upload and version subtitles/captions independently of the video master.
- **NM-L10N-002 (P1):** Subtitle assets record language, format, accessibility type and validation status.
- **NM-L10N-003 (P2):** Workflows can generate draft subtitles using speech recognition, followed by human review before publication.
- **NM-L10N-004 (P2):** Studios can manage localized title descriptions, artwork and metadata.
- **NM-L10N-005 (P2):** Player supports available caption, subtitle and audio-track selection.
- **NM-L10N-006 (P3):** Translation assistance may be offered as an optional workflow; machine output must be labeled and reviewable.

## 5. Non-functional requirements

- **Security:** Tenant isolation, least privilege, scoped signed grants, key rotation, asset encryption, audit trails and penetration testing for vault, embed and screener surfaces.
- **Privacy:** Data minimization, consent and suppression enforcement, purpose limitation, retention controls and export/delete workflows.
- **Reliability:** Graceful degradation of recommendations, analytics and optional integrations; durable workflows for asset processing and financial reconciliation.
- **Performance:** Studio dashboards should use pre-aggregated data; playback authorization must not require a synchronous analytics dependency.
- **Scalability:** Scale media processing, delivery and analytics independently from transactional APIs.
- **Portability:** Studios can export permitted records and asset manifests in documented formats.
- **Cost transparency:** Measure cost per stored GB-month, processed source hour, delivered viewing hour, transaction and active studio.
- **Auditability:** Rights, financial, access and asset lifecycle events must be attributable and queryable.

## 6. Post-MVP release sequence

### Release 2 — Audience and viewer depth
- Studio follows, consent preferences, notification campaigns and basic audience segments.
- Viewer discovery improvements, editorial collections, repeat-viewer reporting and campaign attribution.
- Cross-device watch progress and robust offline metadata synchronization.

### Release 3 — Media operations and external distribution
- Versioned vault, lifecycle/retention controls, detailed access audit and checksum verification.
- Screener campaigns, recipient-bound access and watermarking.
- Embeddable player, branded external storefronts and release scheduling.
- Rights-window alerts, offer bundles and permitted data exports.

### Release 4 — Commercial sophistication
- Studio memberships, paid premieres, early access and bundles.
- Institution/event licensing, group entitlements and payout/revenue-share reporting.
- Advanced campaign attribution, cohort analytics and title-level profitability.

### Release 5 — Distribution network and intelligence
- Festival, campus, community cinema and corporate distribution.
- Partner APIs and channel adapters.
- Localization workflows, cross-studio recommendations and advanced discovery.
- Conditional local caching and multi-CDN/DRM enhancements after validation.

## 7. Acceptance criteria for post-MVP readiness

A capability is not complete merely because its UI exists. It must pass the relevant end-to-end workflow, authorization, audit, failure-recovery and cost tests.

- A studio can create a new asset version without disrupting a live release and can identify which channels reference each version.
- A revoked screener invitation stops new playback authorization and the event is auditable.
- A title becomes unavailable when the applicable rights window expires, including through embeds and active grant renewal.
- A studio can launch a campaign and trace aggregate conversion to the offer and transaction.
- An audience export excludes records without a valid permitted basis and records the export actor, time and scope.
- An institution can receive bounded access for a defined audience, territory and time window.
- A supported client can download, resume, verify, play offline and expire a licensed title.
- A studio can reconcile gross revenue to fees, refunds, rights-holder shares and net payout.
- Optional analytics or recommendation outages do not prevent authorized playback.
- Every feature has documented operational ownership, metrics, support procedures and a cost model.

## 8. Success metrics

- Studio retention and percentage of studios managing multiple active titles.
- Percentage of studio releases using more than one authorized channel.
- Time from title readiness to first revenue.
- Studio net revenue and repeat-purchaser rate.
- Campaign-to-paid-conversion rate.
- Repeat viewers per studio/title.
- Screener invitation-to-completion and review-response rates.
- Asset retrieval success and vault integrity incidents.
- Playback startup time, rebuffer ratio and playback failure rate.
- Offline download completion and offline license failure rate.
- Cost per active studio, title, processed source hour and delivered viewing hour.

## 9. Product risks and mitigations

| Risk | Mitigation |
|---|---|
| Studios do not value advanced workflows enough to pay | Validate each major post-MVP capability with pilot studios and price tests |
| Audience tools become a privacy liability | Consent-first design, suppression enforcement, scoped exports and audit |
| Rights complexity causes unauthorized distribution | Central policy engine, evidence references, expiry automation and deny-by-default |
| Vault costs grow uncontrollably | Storage tiers, lifecycle policies, quotas, cold storage and transparent cost reporting |
| Watermarking/DRM creates false confidence | State limitations clearly; use layered controls and contractual safeguards |
| Institutional workflows become bespoke consulting | Standardize channel, group entitlement and reporting primitives |
| Offline licensing is unreliable | Limit supported clients initially, test clock changes/revocation/expiry and provide clear UX |
| Recommendation systems favor already popular titles | Include editorial controls, exposure diversity and transparent sponsored placement |

## 10. Guiding principle

Every post-MVP feature must strengthen at least one of four durable advantages: **studio operations, rights-aware distribution, permissioned audience relationships, or commercial intelligence**. Viewer-facing features should reinforce those advantages rather than turn ViewAura into an undifferentiated streaming clone.
