package model

import "time"

// One monotonic incoming-message cursor per reader and peer.
type ReadReceipt struct {
	ReaderID, PeerID, MessageID string
	MessageCreatedAt            time.Time
}
