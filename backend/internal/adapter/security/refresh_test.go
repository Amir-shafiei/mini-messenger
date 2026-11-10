package security

import (
	"errors"
	"mini-messenger/internal/model"
	"testing"
)

func TestRefreshSecrets(t *testing.T) {
	secrets := RefreshSecrets{}
	first, hash, err := secrets.Generate()
	if err != nil || len(first) != 43 || len(hash) != 64 || first == hash {
		t.Fatal("invalid secret material")
	}
	second, _, err := secrets.Generate()
	if err != nil || first == second {
		t.Fatal("repeated random token")
	}
	digest, err := secrets.Digest(first)
	if err != nil || digest != hash {
		t.Fatal("unstable hash")
	}
	for _, raw := range []string{"", "bad", first + "="} {
		if _, err = secrets.Digest(raw); !errors.Is(err, model.ErrUnauthorized) {
			t.Fatal("malformed token accepted")
		}
	}
}
