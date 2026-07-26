// Package tf2 is a Team Fortress 2 Game Coordinator client built on
// steamclient. After Connect completes the GC hello/welcome handshake,
// the client mirrors the account's shared-object cache: Backpack and
// AccountInfo answer from that local mirror and stay current as the GC
// pushes changes.
//
// Item operations — Craft, UseItem, DeleteItem, RemoveCrafterName,
// RemoveGifter — are fire-and-forget GC messages; results arrive through
// the corresponding With*Handler callbacks (e.g. WithCraftCompletedHandler,
// WithItemAcquiredHandler) rather than return values, matching how the GC
// actually behaves.
package tf2
