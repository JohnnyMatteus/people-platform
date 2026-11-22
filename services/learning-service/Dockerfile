# syntax=docker/dockerfile:1.7
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
ARG VERSION=dev
# go build baixa só os módulos que o binário importa (não as dependências de teste);
# os cache mounts reaproveitam módulos e compilação entre builds.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/learning-service ./cmd/learning-service

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/learning-service /learning-service
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/learning-service"]
CMD ["serve"]
