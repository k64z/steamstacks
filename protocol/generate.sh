#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TEMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TEMP_DIR"' EXIT

PROTO_REPO="https://github.com/SteamDatabase/Protobufs"
PROTO_DIR="$TEMP_DIR/Protobufs"

echo "Cloning SteamDatabase/Protobufs..."
git clone --depth 1 "$PROTO_REPO" "$PROTO_DIR"

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
    "enums.proto"
)

for proto in "${PROTOS[@]}"; do
    cp "$PROTO_DIR/steam/$proto" "$BUILD_DIR/"
done

cp "$SCRIPT_DIR/Dockerfile" "$BUILD_DIR/"

echo "Building protoc Docker image..."
docker build -t steamstacks-protoc "$BUILD_DIR"

echo "Extracting generated Go files..."
CONTAINER_ID=$(docker create steamstacks-protoc)
for proto in "${PROTOS[@]}"; do
    go_file="${proto%.proto}.pb.go"
    docker cp "$CONTAINER_ID:/build/$go_file" "$SCRIPT_DIR/$go_file"
    echo "  $go_file"
done
docker rm "$CONTAINER_ID" > /dev/null

echo "Done."
