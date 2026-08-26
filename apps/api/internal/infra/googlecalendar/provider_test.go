package googlecalendar

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCodeChallengeUsesS256(t *testing.T) {
	got := CodeChallenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")
	if got != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Fatalf("unexpected PKCE challenge %q", got)
	}
}

func TestEventPayloadDoesNotContainPatientIdentity(t *testing.T) {
	payload := eventWriteResource(time.Now(), time.Now().Add(time.Hour), "Consultorio", uuid.New(), uuid.New())
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	if !strings.Contains(text, `"summary":"Sesión"`) {
		t.Fatalf("generic summary missing: %s", text)
	}
	for _, forbidden := range []string{"patient", "paciente", "client_id", "clientId", "notas clínicas"} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(forbidden)) {
			t.Fatalf("payload leaked %q: %s", forbidden, text)
		}
	}
}
