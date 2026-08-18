# lumo-tamer, packaged for Railway.
#
# Upstream (https://github.com/ZeroTricks/lumo-tamer) ships a Dockerfile, but it
# builds from a local checkout and is configured entirely through a config.yaml
# file that has to exist before the container starts. Railway has neither: there
# is no repo to mount and no way to place a file into the container ahead of
# boot. So this image clones a pinned upstream revision at build time and adds
# an entrypoint that renders config.yaml from environment variables.
#
# Everything below the clone mirrors upstream's own Dockerfile stages.

# Pin upstream so a Railway redeploy can never silently pick up a different
# revision. Bump deliberately, and re-check config.defaults.yaml when you do.
ARG LUMO_TAMER_REPO=https://github.com/ZeroTricks/lumo-tamer.git
ARG LUMO_TAMER_REF=8fd9514db83d5854b5b5c7116f2c3efa9abdc16a

# ============================================================================
# Source stage - clone the pinned revision
# ============================================================================
FROM alpine/git:latest AS source

ARG LUMO_TAMER_REPO
ARG LUMO_TAMER_REF

WORKDIR /src
RUN git clone "$LUMO_TAMER_REPO" . \
    && git checkout --detach "$LUMO_TAMER_REF" \
    && rm -rf .git

# ============================================================================
# Go builder - static proton-auth binary (used by `tamer auth login`)
# ============================================================================
FROM golang:1.24-alpine AS go-builder

WORKDIR /build
COPY --from=source /src/src/auth/login/go ./
RUN CGO_ENABLED=0 go build -o proton-auth

# ============================================================================
# Node builder - compile TypeScript
# ============================================================================
FROM node:22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache python3 py3-setuptools make g++ libc6-compat \
    cairo-dev pango-dev jpeg-dev giflib-dev librsvg-dev

COPY --from=source /src/package.json /src/package-lock.json /src/tsconfig.json ./
COPY --from=source /src/src ./src
COPY --from=source /src/packages ./packages

# playwright is only pulled in by the optional "browser" auth method, which this
# image does not use. Without this, its postinstall downloads a full Chromium
# into an image that can never run it.
ENV PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1

RUN npm ci && npm run build && npm prune --production

# ============================================================================
# Final image
# ============================================================================
FROM node:22-alpine

WORKDIR /app

COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/package.json ./package.json
COPY --from=source /src/config.defaults.yaml ./config.defaults.yaml
COPY --from=go-builder /build/proton-auth ./dist/proton-auth

RUN npm link

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Token vault and the conversation database live here. Mount a Railway volume on
# this path: tokens are rewritten on every refresh (Proton rotates the refresh
# token), so an ephemeral filesystem means re-authenticating after each deploy.
VOLUME ["/app/sessions"]

ENV NODE_ENV=production

EXPOSE 3003

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["server"]
