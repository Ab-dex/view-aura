# ── Build stage ────────────────────────────────────────────────────────────────
FROM node:20-alpine AS builder

WORKDIR /build

COPY package.json package-lock.json* ./
RUN npm ci --ignore-scripts

COPY tsconfig.json ./
COPY src ./src
RUN npm run build

# ── Runtime stage ───────────────────────────────────────────────────────────────
FROM node:20-alpine

RUN addgroup -S email && adduser -S email -G email

WORKDIR /app

# Copy only production dependencies + compiled output
COPY package.json package-lock.json* ./
RUN npm ci --omit=dev --ignore-scripts

COPY --from=builder /build/dist ./dist

USER email

ENV NODE_ENV=production

CMD ["node", "dist/index.js"]