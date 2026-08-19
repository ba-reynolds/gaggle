# syntax=docker/dockerfile:1

# ---- build stage ----
FROM node:24-alpine AS build
WORKDIR /app

# Vite embeds the API base URL at build time. Defaults to the same-origin
# path served by nginx; override with --build-arg if needed.
ARG VITE_API_BASE_URL=/api/v1
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL

# Cache npm install layers
COPY package.json package-lock.json ./
# package.json uses npm's allowScripts for esbuild/@tailwindcss/oxide; npm ci
# honors that field, so no --ignore-scripts here.
RUN npm ci

COPY . .
RUN npm run build

# ---- runtime stage: nginx serving the built app + reverse proxy to API ----
FROM nginx:alpine
RUN apk add --no-cache openssl && rm -f /etc/nginx/conf.d/default.conf

# Replace the stock entrypoint: it first guarantees a TLS cert exists
# (self-signed fallback unless a real cert is already mounted), then boots
# nginx. The stock default.conf is removed so only nginx.conf binds :80/:443.
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY docker-entrypoint.sh /docker-entrypoint.sh
COPY --from=build /app/dist /usr/share/nginx/html

ENTRYPOINT ["/docker-entrypoint.sh"]
EXPOSE 80 443