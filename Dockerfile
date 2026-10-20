# Pull:   docker pull ghcr.io/aaryandhakal22/bday_counter:latest  (built by .github/workflows/docker.yml)
# Build:  docker build -t bday .
# Run:    see compose.yaml (Traefik + Authelia). Without a proxy, set ADMIN_PASSWORD instead of TRUSTED_PROXIES.
# The mic needs HTTPS, which Traefik provides.
FROM golang:1.23-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY *.go ./
COPY web ./web
COPY templates ./templates
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/bday . && mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/bday /bday
COPY --from=build --chown=65532:65532 /out/data /data

ENV DATA_DIR=/data PORT=8080
VOLUME /data
EXPOSE 8080

ENTRYPOINT ["/bday"]
