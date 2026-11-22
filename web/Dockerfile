# syntax=docker/dockerfile:1.7
FROM node:22.14.0-alpine AS build
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
# O build roda o vue-tsc: tipo quebrado não vira imagem. Os padrões apontam para o compose local.
ARG VITE_OIDC_AUTHORITY=http://localhost:8180/realms/people
ARG VITE_OIDC_CLIENT_ID=people-web
ENV VITE_OIDC_AUTHORITY=$VITE_OIDC_AUTHORITY VITE_OIDC_CLIENT_ID=$VITE_OIDC_CLIENT_ID
RUN npm run build

FROM nginx:1.27.4-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/dist /usr/share/nginx/html
EXPOSE 80
HEALTHCHECK --interval=10s --timeout=5s --retries=12 CMD ["wget", "-q", "--spider", "http://127.0.0.1/healthz"]
