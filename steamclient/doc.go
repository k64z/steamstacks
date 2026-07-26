// Package steamclient speaks Steam's Connection Manager (CM) protocol —
// the same protobuf-over-WebSocket (or TCP) session the desktop client
// maintains. A logged-in Client appears online, receives server pushes
// (friend messages, persona states, trade and item notifications, wallet
// updates), and can talk to Game Coordinators for individual games.
//
// Construct a Client with New, then Connect and Login with a refresh
// token obtained from steamsession. Incoming events are delivered through
// the With*Handler options; set handlers before Connect and do not mutate
// them while connected, as they are invoked from the connection's read
// goroutine. The tf2 package builds a Game Coordinator client for Team
// Fortress 2 on top of this package via SendGCMessage and
// WithGCMessageHandler.
package steamclient
