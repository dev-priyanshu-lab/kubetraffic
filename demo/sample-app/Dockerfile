# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS build
WORKDIR /src
# Standard-library only: no go.sum, nothing to download.
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/sample-app ./

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/sample-app /sample-app
EXPOSE 8080
# Numeric UID/GID so Kubernetes runAsNonRoot can verify it (distroless nonroot = 65532).
USER 65532:65532
ENTRYPOINT ["/sample-app"]
