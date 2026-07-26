FROM alpine:3.23

# protoc is an input to checked-in generated code, so it is pinned like any
# other: an unversioned `apk add` would let a future alpine:3.23 patch move
# protoc underneath us and churn all nine .pb.go files on an unrelated run.
# Bumping this is a deliberate act — see docs/decisions.md.
ARG PROTOC_VERSION=31.1-r1
RUN apk add --no-cache \
        "protobuf=$PROTOC_VERSION" \
        "protobuf-dev=$PROTOC_VERSION" \
        go

# Supplied by generate.sh from go.mod's google.golang.org/protobuf so the
# generator and the runtime it targets cannot drift apart.
ARG PROTOBUF_VERSION
ENV GOBIN=/usr/local/bin
RUN test -n "$PROTOBUF_VERSION" || { echo "PROTOBUF_VERSION build-arg is required; run via generate.sh" >&2; exit 1; }
RUN go install "google.golang.org/protobuf/cmd/protoc-gen-go@$PROTOBUF_VERSION"

WORKDIR /build
COPY *.proto ./

# Every .proto copied in maps to this one Go package, so the M flags are
# derived from the files present rather than restated here — generate.sh's
# PROTOS array stays the single source of truth for what gets built.
RUN set -eu; \
    for p in *.proto; do \
        set -- "$@" "--go_opt=M$p=github.com/k64z/steamstacks/protocol"; \
    done; \
    protoc \
        --proto_path=/usr/include \
        --proto_path=. \
        --go_out=. \
        --go_opt=paths=source_relative \
        "$@" \
        *.proto
