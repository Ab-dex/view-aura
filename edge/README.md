# ViewAura Edge Workers

Four Cloudflare Workers that form the global edge layer. All written in TypeScript, deployed via Wrangler v3.

| Worker | Route | Purpose |
|--------|-------|---------|
| `autocomplete` | `viewaura.com/api/v1/search/suggest*` | KV-cached autocomplete (<1ms) |
| `bot-detection` | `viewaura.com/*` | Bot score + ASN block + rate limit |
| `geo-routing` | `viewaura.com/*` | Route to nearest healthy origin |
| `ab-testing` | `viewaura.com/*` | Sticky variant assignment |

Workers execute in the order: geo-routing → bot-detection → ab-testing → autocomplete (autocomplete is a terminal handler for its route; the others are middleware that call `fetch(request)` to pass through).

---

## Prerequisites

```bash
npm install -g wrangler@3
wrangler login
```

Node.js >= 18 required.

---

## Local development

Each worker runs independently in a local Wrangler dev server backed by a real Cloudflare account (for KV access):

```bash
npm install

# Autocomplete (port 8787)
npm run dev:autocomplete

# Bot detection (port 8788)
npm run dev:bot

# Geo-routing (port 8789)
npm run dev:geo

# A/B testing (port 8790)
npm run dev:ab
```

Test with curl:
```bash
# Autocomplete
curl "http://localhost:8787/api/v1/search/suggest?q=inc"

# Bot detection (simulate low bot score via CF-Bot-Score header in local mode)
curl -H "User-Agent: python-requests/2.31" http://localhost:8788/api/v1/movies

# A/B testing (check the X-AB-Experiments header on the response)
curl -v http://localhost:8790/
```

---

## First-time setup per worker

### 1. Create KV namespaces

```bash
# Autocomplete
wrangler kv:namespace create SUGGEST_CACHE
wrangler kv:namespace create SUGGEST_CACHE --preview

# Bot detection
wrangler kv:namespace create RATE_LIMIT
wrangler kv:namespace create RATE_LIMIT --preview

# Geo-routing
wrangler kv:namespace create ORIGIN_HEALTH
wrangler kv:namespace create ORIGIN_HEALTH --preview

# A/B testing
wrangler kv:namespace create AB_ASSIGNMENTS
wrangler kv:namespace create AB_ASSIGNMENTS --preview
```

Copy the returned `id` and `preview_id` values into each `wrangler.toml`.

### 2. Set secrets

```bash
# Autocomplete worker — internal API key for the search service
wrangler secret put SEARCH_API_KEY --config autocomplete/wrangler.toml
```

### 3. Seed origin health (geo-routing)

The health-check cron runs every minute. To unblock first deployment:
```bash
wrangler kv:key put "us-east-1" '"healthy"' --binding ORIGIN_HEALTH --config geo-routing/wrangler.toml
wrangler kv:key put "eu-west-1" '"healthy"' --binding ORIGIN_HEALTH --config geo-routing/wrangler.toml
wrangler kv:key put "ap-southeast-1" '"healthy"' --binding ORIGIN_HEALTH --config geo-routing/wrangler.toml
```

---

## Deployment

```bash
# Type-check all workers first
npm run typecheck

# Deploy all at once
npm run deploy:all

# Deploy individually
wrangler deploy --config autocomplete/wrangler.toml
wrangler deploy --config bot-detection/wrangler.toml
wrangler deploy --config geo-routing/wrangler.toml
wrangler deploy --config ab-testing/wrangler.toml
```

Workers deploy globally to all Cloudflare PoPs within ~30 seconds of `wrangler deploy`.

---

## How the workers interact

All four workers are deployed to the same zone (`viewaura.com`). Cloudflare routes a request through them based on route matching. Workers that are not terminal (autocomplete terminates; the others call `fetch()`) form a middleware chain:

```
Request
  → geo-routing (selects origin, rewrites URL)
  → bot-detection (checks bot score, rate limits)
  → ab-testing (assigns variant, injects X-AB-Experiments header)
  → autocomplete (KV cache for /suggest routes only)
  → Origin (Go API behind Kong)
```

In practice, Cloudflare executes only one worker per request (the first matching route). To achieve middleware chaining, each non-terminal worker calls `fetch(modifiedRequest)` which re-runs the chain. The order is controlled by route specificity and worker execution order in the Cloudflare dashboard.

---

## A/B testing — adding a new experiment

1. Add the experiment to `shared/types.ts` `EXPERIMENTS` array:

```typescript
{
  id:      "my_new_experiment",
  name:    "Description of what is being tested",
  weightA: 0.5,    // 50% variant A, 50% variant B
  active:  true,
}
```

2. Deploy the ab-testing worker (`wrangler deploy`).
3. In the Go API, read the `X-AB-Experiments` header:

```go
experiments := c.GetHeader("X-AB-Experiments")
// experiments = "home_feed_layout=b,my_new_experiment=a"
variant := parseVariant(experiments, "my_new_experiment")  // "a" | "b" | "control"
```

4. Log the variant with each event so analytics can split by variant.

---

## Module layout

```
edge/
├── package.json        — workspace root, shared dev dependencies
├── tsconfig.json       — shared TypeScript config
├── shared/
│   └── types.ts        — ORIGINS, COUNTRY_TO_REGION, EXPERIMENTS, BLOCKED_ASNS
├── autocomplete/
│   ├── wrangler.toml   — KV binding, route, TTL vars
│   └── index.ts        — KV read → origin proxy → KV write (ctx.waitUntil)
├── bot-detection/
│   ├── wrangler.toml   — KV binding, rate limit vars
│   └── index.ts        — CF bot score + ASN block + UA heuristics + KV rate limit
├── geo-routing/
│   ├── wrangler.toml   — KV binding, origin URLs, health check config
│   └── index.ts        — Country → region → health check → proxy + Cron health poller
└── ab-testing/
    ├── wrangler.toml   — KV binding, cookie config
    └── index.ts        — JWT sub / session cookie → FNV-1a hash → variant → KV store
```