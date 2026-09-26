package ingestion

import (
	"math"
	"testing"
)

func TestTranscriptContractRejectsInvalidSegments(t *testing.T) {
	valid := TranscriptContent{Text: "Contenido sintético.", Language: "es", Engine: "faster-whisper", Model: "synthetic-test", EngineVersion: "1.0", ConfigurationHash: Hash("config"), Segments: []Segment{{Start: 0, End: 2, Text: "Contenido sintético."}}}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, seg := range []Segment{{Start: -1, End: 2, Text: "x"}, {Start: 2, End: 1, Text: "x"}, {Start: 0, End: math.NaN(), Text: "x"}, {Start: 0, End: 2, Text: ""}} {
		bad := valid
		bad.Segments = []Segment{seg}
		if bad.Validate() == nil {
			t.Fatal("accepted invalid segment")
		}
	}
	bad := valid
	bad.Text = "invented mismatch"
	if bad.Validate() == nil {
		t.Fatal("accepted text inconsistent with segments")
	}
	bad = valid
	bad.ConfigurationHash = "not-a-hash"
	if bad.Validate() == nil {
		t.Fatal("accepted missing provenance")
	}
}
