package transcriber

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"
	"sessionflow/apps/api/internal/usecase/ingestion"
	"sessionflow/apps/api/internal/usecase/transcription"
)

func (p *Provider) TranscribeDurable(ctx context.Context, audio []byte, format string, requestID uuid.UUID, configurationHash string) (ingestion.TranscriptContent, error) {
	if requestID == uuid.Nil || !ingestion.ValidHash(configurationHash) || len(audio) == 0 || len(audio) > 25*1024*1024 {
		return ingestion.TranscriptContent{}, errors.New("invalid transcription request")
	}
	switch format {
	case "wav", "webm", "ogg", "mp4", "m4a":
	default:
		return ingestion.TranscriptContent{}, errors.New("invalid audio format")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/transcriptions", bytes.NewReader(audio))
	if err != nil {
		return ingestion.TranscriptContent{}, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Audio-Format", format)
	req.Header.Set("X-Request-ID", requestID.String())
	req.Header.Set("X-Configuration-Hash", configurationHash)
	req.Header.Set("X-Language", "es")
	response, err := p.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ingestion.TranscriptContent{}, ctx.Err()
		}
		return ingestion.TranscriptContent{}, transcription.ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusConflict {
		return ingestion.TranscriptContent{}, transcription.ErrBusy
	}
	if response.StatusCode != http.StatusOK {
		return ingestion.TranscriptContent{}, transcription.ErrUnavailable
	}
	var body struct {
		Contract  string                      `json:"contract"`
		RequestID uuid.UUID                   `json:"request_id"`
		Content   ingestion.TranscriptContent `json:"content"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 2*1024*1024))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&body); err != nil {
		return ingestion.TranscriptContent{}, errors.New("invalid transcription response")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || body.Contract != "local-transcription-v1" || body.RequestID != requestID || body.Content.ConfigurationHash != configurationHash {
		return ingestion.TranscriptContent{}, errors.New("transcription provenance mismatch")
	}
	if err := body.Content.Validate(); err != nil {
		return ingestion.TranscriptContent{}, err
	}
	return body.Content, nil
}
