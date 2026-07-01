# Web frontend Dockerfile — multi-stage build for the React/Vite SPA.
#
# Build context: repository root (so we can read web/package.json and the
# sibling backend if needed for proxy URL injection at build time).
#
# Build args:
#   VITE_API_BASE   API base URL baked into the bundle (default: same-origin /api)
#
# Example:
#   docker build -f web/Dockerfile \
#     --build-arg VITE_API_BASE=/api \
#     -t bidwriter/web:latest .

ARG NODE_VERSION=20

# ---------------------------------------------------------------------------
# Stage 1: install dependencies (cached layer)
# ---------------------------------------------------------------------------
FROM node:${NODE_VERSION}-alpine AS deps
WORKDIR /app
# Copy lockfile first so this layer is cached when only source changes.
COPY web/package.json web/pnpm-lock.yaml* ./
RUN corepack enable \
 && (test -f pnpm-lock.yaml && pnpm install --frozen-lockfile || npm ci)

# ---------------------------------------------------------------------------
# Stage 2: build the production bundle
# ---------------------------------------------------------------------------
FROM node:${NODE_VERSION}-alpine AS builder
WORKDIR /app

ARG VITE_API_BASE=/api
ENV VITE_API_BASE=${VITE_API_BASE}

COPY --from=deps /app/node_modules ./node_modules
COPY web/ ./
RUN corepack enable \
 && (test -f pnpm-lock.yaml && pnpm run build || npm run build)

# ---------------------------------------------------------------------------
# Stage 3: serve with nginx (tiny, well-known, has health endpoint)
# ---------------------------------------------------------------------------
FROM nginx:1.27-alpine

# Drop default config; we ship our own that (a) serves the SPA, (b) proxies
# /api to api-gateway, (c) exposes /healthz for compose healthcheck.
RUN rm /etc/nginx/conf.d/default.conf
COPY web/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html

# Lightweight healthcheck matching the one in the compose file.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost/healthz || exit 1

EXPOSE 80
