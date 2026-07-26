#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TEMP_DIR="$(mktemp -d)"
CONTAINER_ID=""
# The container is created mid-script and removed at the end; without it in
# the trap, any failure in between (a docker cp for a proto upstream renamed,
# say) aborts under set -e and orphans a container pinning the image layers.
cleanup() {
    rm -rf "$TEMP_DIR"
    [ -n "$CONTAINER_ID" ] && docker rm "$CONTAINER_ID" > /dev/null 2>&1
    return 0
}
trap cleanup EXIT

PROTO_REPO="https://github.com/SteamDatabase/Protobufs"
PROTO_DIR="$TEMP_DIR/Protobufs"
REV_FILE="$SCRIPT_DIR/UPSTREAM_REV"

# protoc-gen-go must match the google.golang.org/protobuf the library links
# against, or generated code and runtime disagree about protoimpl versions.
# Derived from go.mod rather than restated in the Dockerfile so the two
# cannot drift — nothing in CI would catch it if they did.
# Handles both `require google.golang.org/protobuf v1.2.3` and the
# indented form inside a require ( ... ) block.
PROTOBUF_VERSION="$(awk '{ for (i = 1; i < NF; i++) if ($i == "google.golang.org/protobuf") { print $(i + 1); exit } }' "$SCRIPT_DIR/../go.mod")"
if [ -z "$PROTOBUF_VERSION" ]; then
    echo "error: could not read google.golang.org/protobuf version from go.mod" >&2
    exit 1
fi
echo "protoc-gen-go version (from go.mod): $PROTOBUF_VERSION"

echo "Cloning SteamDatabase/Protobufs..."
git clone --depth 1 "$PROTO_REPO" "$PROTO_DIR"

# Record which upstream revision produced this output — the clone is
# throwaway, so without this there is no way to tell later what the
# committed .pb.go files were generated from. Written to a tracked file
# (not just printed) so the two endpoints of a future `git diff
# <old-rev>..<new-rev>` against the upstream repo are recoverable.
UPSTREAM_REV="$(git -C "$PROTO_DIR" rev-parse HEAD)"
echo "Upstream revision: $UPSTREAM_REV"

BUILD_DIR="$TEMP_DIR/build"
mkdir -p "$BUILD_DIR"

PROTOS=(
    "steammessages_base.proto"
    "steammessages_unified_base.steamclient.proto"
    "steammessages_auth.steamclient.proto"
    "steammessages_clientserver_login.proto"
    "steammessages_clientserver_friends.proto"
    "encrypted_app_ticket.proto"
    "steammessages_clientserver.proto"
    "steammessages_clientserver_2.proto"
    "enums.proto"
)

for proto in "${PROTOS[@]}"; do
    cp "$PROTO_DIR/steam/$proto" "$BUILD_DIR/"
done

cp "$SCRIPT_DIR/Dockerfile" "$BUILD_DIR/"

echo "Building protoc Docker image..."
docker build \
    --build-arg "PROTOBUF_VERSION=$PROTOBUF_VERSION" \
    -t steamstacks-protoc "$BUILD_DIR"

echo "Extracting generated Go files..."
CONTAINER_ID=$(docker create steamstacks-protoc)
for proto in "${PROTOS[@]}"; do
    go_file="${proto%.proto}.pb.go"
    docker cp "$CONTAINER_ID:/build/$go_file" "$SCRIPT_DIR/$go_file"
    echo "  $go_file"
done

printf '%s\n' "$UPSTREAM_REV" > "$REV_FILE"

echo "Done. Generated from SteamDatabase/Protobufs $UPSTREAM_REV"
