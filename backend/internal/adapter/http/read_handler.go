package httpadapter

import (
	"mini-messenger/internal/model"
	"mini-messenger/internal/service/auth"
	"net/http"
	"time"
)

type receiptJSON struct {
	ReaderID         string    `json:"reader_id"`
	PeerID           string    `json:"peer_id"`
	MessageID        string    `json:"message_id"`
	MessageCreatedAt time.Time `json:"message_created_at"`
}

func receiptResponse(r model.ReadReceipt) any {
	if r.MessageID == "" {
		return nil
	}
	return receiptJSON{
		ReaderID:         r.ReaderID,
		PeerID:           r.PeerID,
		MessageID:        r.MessageID,
		MessageCreatedAt: r.MessageCreatedAt,
	}
}
func (h *Handler) readHandler(w http.ResponseWriter, r *http.Request, p auth.Principal) {
	peer := r.PathValue("user_id")
	if !validUUID(peer) {
		writeError(w, 400, "invalid_input")
		return
	}
	var receipt model.ReadReceipt
	var err error
	if r.Method == http.MethodPost {
		var in struct {
			MessageID string `json:"message_id"`
		}
		if !readJSON(w, r, &in) {
			return
		}
		if !validUUID(in.MessageID) {
			writeError(w, 400, "invalid_input")
			return
		}
		receipt, err = h.reads.Mark(r.Context(), p.UserID, peer, in.MessageID)
	} else {
		receipt, err = h.reads.PeerRead(r.Context(), p.UserID, peer)
	}
	if err != nil {
		domainError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"receipt": receiptResponse(receipt)})
}
