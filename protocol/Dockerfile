FROM alpine:3.21

RUN apk add --no-cache protobuf protobuf-dev go

ENV GOBIN=/usr/local/bin
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.8

WORKDIR /build
COPY *.proto ./

RUN protoc \
    --proto_path=/usr/include \
    --proto_path=. \
    --go_out=. \
    --go_opt=paths=source_relative \
    --go_opt=Msteammessages_base.proto=github.com/k64z/steamstacks/protocol \
    --go_opt=Msteammessages_unified_base.steamclient.proto=github.com/k64z/steamstacks/protocol \
    --go_opt=Msteammessages_auth.steamclient.proto=github.com/k64z/steamstacks/protocol \
    --go_opt=Msteammessages_clientserver_login.proto=github.com/k64z/steamstacks/protocol \
    --go_opt=Menums.proto=github.com/k64z/steamstacks/protocol \
    steammessages_base.proto \
    steammessages_unified_base.steamclient.proto \
    steammessages_auth.steamclient.proto \
    steammessages_clientserver_login.proto \
    enums.proto
