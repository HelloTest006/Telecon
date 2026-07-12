# Multi-stage build for COE server components (AGPL-3.0)
# Images: ka (Key Authority), coe-signal (WebRTC signaling)
FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /out/ka ./cmd/ka \
 && CGO_ENABLED=0 go build -ldflags "-s -w" -o /out/coe-signal ./cmd/coe-signal \
 && CGO_ENABLED=0 go build -ldflags "-s -w" -o /out/ka-check ./cmd/ka-check \
 && CGO_ENABLED=0 go build -ldflags "-s -w" -o /out/coe-admin ./cmd/coe-admin

FROM gcr.io/distroless/static-debian12:nonroot AS ka
WORKDIR /data
COPY --from=build /out/ka /ka
COPY --from=build /out/ka-check /ka-check
COPY --from=build /out/coe-admin /coe-admin
# Default paths inside volume
ENV COE_KA_LISTEN=0.0.0.0:8443 \
    COE_KA_MASTER_FILE=/data/master.key \
    COE_KA_REGISTRY=/data/registry.json \
    COE_KA_TLS_CERT=/data/tls/server.crt \
    COE_KA_TLS_KEY=/data/tls/server.key \
    COE_KA_TLS_HOSTS=localhost,127.0.0.1
VOLUME ["/data"]
EXPOSE 8443
USER nonroot:nonroot
ENTRYPOINT ["/ka"]

FROM gcr.io/distroless/static-debian12:nonroot AS signal
COPY --from=build /out/coe-signal /coe-signal
ENV COE_SIGNAL_LISTEN=0.0.0.0:8450
EXPOSE 8450
USER nonroot:nonroot
ENTRYPOINT ["/coe-signal"]
