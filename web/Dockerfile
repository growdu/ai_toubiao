# Web frontend Dockerfile — multi-stage build for the React/Vite SPA.
#
# Build context: repository root (so we can read web/package.json and the
# sibling backend if needed for proxy URL injection at build time).
#
# Build args:
#   VITE_API_BASE   API base URL baked into the bundle (default: same-origin /api)
#
# Requires: Docker with BuildKit (the legacy builder is deprecated in
# Docker 28+ and removed in 29+). On a clean host:
#   docker buildx create --use
#   docker buildx build -f web/Dockerfile -t bidwriter/web:latest .
# Plain `docker build` also works on Docker 27 and earlier; on 29+ the
# default builder errors with "no build stage in current context" and you
# must either install the buildx plugin or use the older Docker engine.
#
# Example:
#   docker build -f web/Dockerfile \
#     --build-arg VITE_API_BASE=/api \
#     -t bidwriter/web:latest .

ARG NODE_VERSION=20

# Pin pnpm to a version compatible with Node 20. corepack's default behaviour
# is to fetch the latest pnpm (v11+ as of mid-2026), which requires Node 22+
# at runtime and breaks `corepack enable && pnpm install` on the node:20-alpine
# base image (this was the original pnpm-11-vs-node-20 failure). Locked pnpm@9
# works on Node 18 / 20 / 22 and is what the lockfile in this repo was generated
# against. If the lockfile is missing we fall back to npm ci.
ARG PNPM_VERSION=9.15.4
ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"

# ---------------------------------------------------------------------------
# Stage 1: install dependencies (cached layer)
# ---------------------------------------------------------------------------
FROM node:${NODE_VERSION}-alpine AS deps
WORKDIR /app

# Copy both manifests so the layer is busted when EITHER changes.
COPY web/package.json web/pnpm-lock.yaml* web/package-lock.json* ./

# Use pnpm when the lockfile is present (the normal case for this repo),
# otherwise npm ci. We do NOT call `corepack enable` because that pulls the
# latest pnpm on first run — instead we install a specific version via the
# standalone package, which is reproducible across base image rebuilds.
RUN if [ -f pnpm-lock.yaml ]; then \
      npm install -g "[email protected]" --silent && \
      pnpm install --frozen-lockfile; \
    else \
      npm ci; \
    fi

# ---------------------------------------------------------------------------
# Stage 2: build the production bundle
# ---------------------------------------------------------------------------
FROM node:${NODE_VERSION}-alpine AS builder
WORKDIR /app

ARG VITE_API_BASE=/api
ENV VITE_API_BASE=${VITE_API_BASE}

COPY --from=deps /app/node_modules ./node_modules
COPY web/ ./

RUN if [ -f pnpm-lock.yaml ]; then \
      npm install -g "[email protected]" --silent && \
      pnpm run build; \
    else \
      npm run build; \
    fi

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
