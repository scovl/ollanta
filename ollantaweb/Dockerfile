FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Copy workspace manifest and all module files first for better layer caching.
COPY go.work go.work.sum* ./
COPY ollantacore/go.mod    ollantacore/go.sum*    ./ollantacore/
COPY ollantaengine/go.mod  ollantaengine/go.sum*  ./ollantaengine/
COPY ollantastore/go.mod   ollantastore/go.sum*   ./ollantastore/
COPY ollantaweb/go.mod     ollantaweb/go.sum*     ./ollantaweb/
# go.work references all workspace modules; copy go.mod stubs so the workspace resolves
COPY ollantaparser/go.mod  ollantaparser/go.sum*  ./ollantaparser/
COPY ollantarules/go.mod   ollantarules/go.sum*   ./ollantarules/
COPY ollantascanner/go.mod ollantascanner/go.sum* ./ollantascanner/
COPY adapter/go.mod     adapter/go.sum*     ./adapter/
COPY application/go.mod application/go.sum* ./application/
COPY domain/go.mod      domain/go.sum*      ./domain/

RUN go work sync || true

# Copy only the modules ollantaweb needs (no CGO, no tree-sitter).
COPY ollantacore/   ./ollantacore/
COPY ollantaengine/ ./ollantaengine/
COPY ollantastore/  ./ollantastore/
COPY ollantaweb/    ./ollantaweb/

ENV CGO_ENABLED=0 GOOS=linux
RUN go build -trimpath -ldflags="-s -w" -o /ollantaweb ./ollantaweb/cmd/ollantaweb

FROM gcr.io/distroless/static-debian12:nonroot AS final

COPY --from=builder /ollantaweb /ollantaweb
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 8080
ENTRYPOINT ["/ollantaweb"]
