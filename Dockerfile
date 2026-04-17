# syntax=docker/dockerfile:1.7

# ── Stage 1: builder ──────────────────────────────────────────────────────────
# golang:1.21-bookworm already ships gcc, so CGO works out of the box.
FROM golang:1.21-bookworm AS builder

WORKDIR /src

# Copy workspace descriptor first (best cache-layer ordering).
COPY go.work go.work.sum* ./

# Copy each module's dependency manifests before the source so that
# `go mod download` is only re-run when dependencies actually change.
COPY ollantacore/go.mod                       ollantacore/
COPY ollantaengine/go.mod                     ollantaengine/
COPY ollantaparser/go.mod ollantaparser/go.sum ollantaparser/
COPY ollantarules/go.mod                      ollantarules/
COPY ollantascanner/go.mod ollantascanner/go.sum ollantascanner/

# Download all modules (BuildKit cache keeps this layer warm across builds).
RUN --mount=type=cache,target=/root/go/pkg/mod \
    go mod download

# Copy the rest of the source tree.
COPY . .

# Compile a fully-static binary so the runtime image needs no C libs.
# -trimpath removes local build paths from the binary.
RUN --mount=type=cache,target=/root/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux \
    go build \
      -trimpath \
      -ldflags="-s -w -extldflags '-static'" \
      -o /ollanta \
      ./ollantascanner/cmd/ollanta


# ── Stage 2: runtime ─────────────────────────────────────────────────────────
# distroless/static-debian12 is the minimal base for fully-static binaries.
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.source="https://github.com/scovl/ollanta"
LABEL org.opencontainers.image.description="Ollanta static analysis scanner"

COPY --from=builder /ollanta /usr/local/bin/ollanta

# /project is the default mount point for the directory being scanned.
VOLUME ["/project"]

# The report is written to /project/.ollanta/ by default.
# Expose the UI port (only active when -serve is passed).
EXPOSE 7777

ENTRYPOINT ["ollanta"]
# Default: scan /project and open the UI on 0.0.0.0:7777 (container-friendly).
CMD ["-project-dir", "/project", "-project-key", "project", "-format", "all", \
     "-serve", "-bind", "0.0.0.0", "-port", "7777"]
