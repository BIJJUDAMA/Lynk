# syntax=docker/dockerfile:1

# Stage 1: Base image with official Node.js 20 LTS slim
FROM node:20-slim AS base
WORKDIR /app

# Stage 2: Dependencies layer with BuildKit npm cache mount
FROM base AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci

# Stage 3: Builder layer with BuildKit compiler cache mounts
FROM base AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .

ENV NEXT_TELEMETRY_DISABLED=1
ENV NODE_ENV=production

# Public environment variables available at build time
ARG NEXT_PUBLIC_API_URL=http://localhost:8080
ARG NEXT_PUBLIC_WEBSITE_URL=http://localhost:3000
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_WEBSITE_URL=$NEXT_PUBLIC_WEBSITE_URL

# Build with cached npm modules and Next.js compiler artifacts
RUN --mount=type=cache,target=/root/.npm \
    --mount=type=cache,target=/app/.next/cache \
    npm run build

# Stage 4: Minimal production runner (Kubernetes PodSecurityStandard compliant)
FROM base AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

# Dedicated non-root user (UID 1001)
RUN groupadd --system --gid 1001 nodejs && \
    useradd --system --uid 1001 -g nodejs nextjs

# Copy static assets and public directory
COPY --from=builder /app/public ./public

# Ensure write permissions for Next.js prerender cache
RUN mkdir .next && chown nextjs:nodejs .next

# Copy standalone server bundles and static assets
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs

EXPOSE 3000

# Native HTTP health check for Docker and Kubernetes compatibility
HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
  CMD node -e "require('http').get('http://127.0.0.1:3000/', (r) => {if (r.statusCode >= 200 && r.statusCode < 400) process.exit(0); else process.exit(1);}).on('error', () => process.exit(1));"

# PID 1 exec-form ensures OS signals (SIGTERM) are caught for graceful teardown
CMD ["node", "server.js"]
