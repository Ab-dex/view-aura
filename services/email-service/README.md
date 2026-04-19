# email-service

Node.js transactional email service. Consumes Kafka events and sends emails via SendGrid using React Email templates.

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Node.js | ≥ 20 | [nodejs.org](https://nodejs.org) |
| npm | ≥ 10 | bundled with Node.js |

No native build dependencies — unlike the Rust search service, this installs cleanly with `npm ci`.

---

## Local setup

### 1. Install dependencies

```bash
npm ci
```

### 2. Configure

```bash
cp .env.example .env
```

Edit `.env` and set at minimum:

```env
SENDGRID_API_KEY=SG.your_key_here
```

To run without sending real emails, set `SENDGRID_API_KEY=SG.placeholder` and comment out the `await sgMail.send(...)` call in `src/mailer/index.ts`. The service will log what it _would_ send.

To run without Kafka (HTTP-only mode), leave `KAFKA_BROKERS` empty.

### 3. Start Kafka (optional — use monolith compose)

```bash
cd ../../
docker compose -f deployments/docker/docker-compose.yml up -d kafka kafka-init
```

### 4. Run in development mode (hot reload)

```bash
npm run dev
```

### 5. Build and run compiled output

```bash
npm run build
npm start
```

---

## Preview emails in a browser

React Email ships a preview server. To preview all templates without sending:

```bash
npx react-email dev --dir src/templates --port 3001
# → http://localhost:3001
```

---

## Events consumed

| Kafka topic | Event type | Email sent |
|-------------|-----------|------------|
| `user.events` | `user.registered` | Welcome email |
| `payment.events` | `payment.payment_succeeded` | Receipt |
| `payment.events` | `payment.payment_failed` | Payment failure + update CTA |
| `payment.events` | `payment.subscription_cancelled` | Cancellation confirmation |
| `uploads.completed` | `upload.asset_published` | "Your video is live" |
| `moderation.decided` | `moderation.decided` | Approval / rejection / escalation |

### Email address requirement

Payment, upload, and moderation events carry `user_id` but not the user's email address. The email service needs the address to deliver. Two approaches:

1. **Denormalise into the event** — the Go monolith adds `email` and `display_name` fields when publishing. This is the current expectation (see `if (!p.email) return` guards).
2. **Internal API lookup** — call `GET /internal/users/{user_id}/email` before sending. Implement this in `consumer/index.ts` when the monolith exposes an internal route.

---

## Module layout

```
src/
├── index.ts           — entrypoint, graceful shutdown
├── logger.ts          — pino logger
├── config/
│   └── index.ts       — zod-validated config from env
├── mailer/
│   └── index.ts       — SendGrid wrapper, React Email render
├── consumer/
│   └── index.ts       — KafkaJS consumer, event dispatcher
└── templates/
    ├── base.tsx        — shared layout (logo, footer, unsubscribe)
    ├── welcome.tsx
    ├── payment-receipt.tsx
    ├── payment-failed.tsx
    ├── upload-complete.tsx
    ├── subscription-cancelled.tsx
    └── moderation-decision.tsx
```

---

## Adding a new email template

1. Create `src/templates/my-template.tsx` extending `<BaseEmail>`.
2. Add the event type to the `switch` in `src/consumer/index.ts`.
3. Add the topic to `TOPICS` if it's not already subscribed.
4. Preview with `npx react-email dev`.

---

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NODE_ENV` | `local` | `local` \| `staging` \| `production` |
| `LOG_LEVEL` | `info` | `trace` \| `debug` \| `info` \| `warn` \| `error` |
| `KAFKA_BROKERS` | `""` | Comma-separated; empty disables consumer |
| `KAFKA_GROUP_ID` | `email-service` | Consumer group ID |
| `SENDGRID_API_KEY` | — | **Required.** SendGrid API key |
| `FROM_EMAIL` | `hello@viewaura.com` | Sender address |
| `FROM_NAME` | `ViewAura` | Sender display name |
| `APP_URL` | `https://viewaura.com` | Base URL for email links |
| `UNSUBSCRIBE_URL` | `https://viewaura.com/unsubscribe` | Footer unsubscribe link |
| `LOGO_URL` | `https://viewaura.com/logo.png` | Header logo URL |