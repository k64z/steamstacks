// Package protocol contains protobuf bindings generated from Valve's
// published Steam message definitions (SteamDatabase/Protobufs). It is
// consumed by steamclient and tf2; most users never import it directly.
//
// Regenerate with generate.sh, which runs protoc in the pinned Docker
// image described by the adjacent Dockerfile.
package protocol
