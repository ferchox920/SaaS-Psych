package sessionnote

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCanViewPrivateNote(t *testing.T) {
	authorID := uuid.New()
	note := Entity{
		ID:           uuid.New(),
		AuthorUserID: authorID,
		IsPrivate:    true,
	}

	if !CanView(note, authorID, "member") {
		t.Fatalf("author should view private note")
	}
	if !CanView(note, uuid.New(), "owner") {
		t.Fatalf("owner should view private note")
	}
	if CanView(note, uuid.New(), "member") {
		t.Fatalf("member should not view other private note")
	}
}

func TestNewEntityRequiresBody(t *testing.T) {
	_, err := NewEntity(uuid.New(), uuid.New(), uuid.New(), "   ", true, time.Now())
	if err == nil {
		t.Fatalf("expected validation error")
	}
}
