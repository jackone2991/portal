# syntax=docker/dockerfile:1.7
# Production-grade Next.js image. Uses standalone output for a small final layer.

FROM node:22-alpine AS deps
WORKDIR /app
RUN npm install -g pnpm@9
COPY package.json pnpm-lock.yaml* ./
RUN --mount=type=cache,id=pnpm,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile || pnpm install

FROM node:22-alpine AS builder
WORKDIR /app
RUN npm install -g pnpm@9
COPY --from=deps /app/node_modules ./node_modules
COPY . .
# NEXT_PUBLIC_* are inlined at build time — they must be ENV before `pnpm build`.
# docker-compose passes these as build args; defaults here cover direct builds.
ARG NEXT_PUBLIC_TEMPLATE_VERSION=v1
ARG NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
ARG NEXT_PUBLIC_WEATHER_LAT=
ARG NEXT_PUBLIC_WEATHER_LON=
ARG NEXT_PUBLIC_WEATHER_PLACE=
ENV NEXT_PUBLIC_TEMPLATE_VERSION=$NEXT_PUBLIC_TEMPLATE_VERSION
ENV NEXT_PUBLIC_API_BASE_URL=$NEXT_PUBLIC_API_BASE_URL
ENV NEXT_PUBLIC_WEATHER_LAT=$NEXT_PUBLIC_WEATHER_LAT
ENV NEXT_PUBLIC_WEATHER_LON=$NEXT_PUBLIC_WEATHER_LON
ENV NEXT_PUBLIC_WEATHER_PLACE=$NEXT_PUBLIC_WEATHER_PLACE
ENV NEXT_TELEMETRY_DISABLED=1
RUN pnpm build

FROM node:22-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
RUN addgroup -S nodejs && adduser -S nextjs -G nodejs

COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static
COPY --from=builder --chown=nextjs:nodejs /app/public ./public

USER nextjs
EXPOSE 3000
ENV HOSTNAME=0.0.0.0
CMD ["node", "server.js"]
