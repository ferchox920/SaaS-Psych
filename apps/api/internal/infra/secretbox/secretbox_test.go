package secretbox

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	box, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := box.Seal("refresh-token")
	if err != nil {
		t.Fatal(err)
	}
	if sealed == "refresh-token" {
		t.Fatal("plaintext token leaked")
	}
	opened, err := box.Open(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if opened != "refresh-token" {
		t.Fatalf("got %q", opened)
	}
}
