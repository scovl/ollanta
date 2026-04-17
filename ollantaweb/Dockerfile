FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Copy workspace manifest and all module files first for better layer caching
COPY go.work go.work.sum* ./
COPY ollantacore/go.mod ollantacore/go.sum* ./ollantacore/
COPY ollantaengine/go.mod ollantaengine/go.sum* ./ollantaengine/
COPY ollantastore/go.mod ollantastore/go.sum* ./ollantastore/
COPY ollantaweb/go.mod ollantaweb/go.sum* ./ollantaweb/

RUN go work sync || true

# Copy all source (ollantaweb does NOT use CGO)
COPY ollantacore/ ./ollantacore/
COPY ollantaengine/ ./ollantaengine/
COPY ollantastore/ ./ollantastore/
COPY ollantaweb/ ./ollantaweb/

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -o /ollantaweb ./ollantaweb/cmd/ollantaweb

FROM gcr.io/distroless/static-debian12 AS final

COPY --from=builder /ollantaweb /ollantaweb
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 8080
ENTRYPOINT ["/ollantaweb"]
