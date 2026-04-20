# ── Build stage ────────────────────────────────────────────────────────────────
FROM node:22-alpine AS builder
WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm ci --ignore-scripts

COPY tsconfig.json ./
COPY src ./src
RUN npm run build

# Prune dev dependencies for smaller runtime image
RUN npm ci --omit=dev --ignore-scripts

# ── Runtime stage ───────────────────────────────────────────────────────────────
FROM node:22-alpine AS runtime

RUN addgroup -S bff && adduser -S bff -G bff
WORKDIR /app

COPY --from=builder /app/dist        ./dist
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json ./

ENV NODE_ENV=production
USER bff
EXPOSE 7001

CMD ["node", "dist/index.js"]