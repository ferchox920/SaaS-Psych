package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"sessionflow/apps/api/internal/usecase/ingestion"
)

type ClinicalIngestionHandler struct {
	health                          func(context.Context) map[string]any
	service                         *ingestion.Service
	maxAudioBytes                   int64
	transcriptionHash, analysisHash string
}

func (h *ClinicalIngestionHandler) Jobs(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	out, err := h.service.ListJobs(c.Request().Context(), t, id, p.UserID)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(200, map[string]any{"items": out})
}

func (h *ClinicalIngestionHandler) DeleteTranscript(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	out, err := h.service.DeleteTranscript(c.Request().Context(), t, id, p.UserID)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(200, out)
}

func NewClinicalIngestionHandler(s *ingestion.Service, max int64, trHash, analysisHash string) *ClinicalIngestionHandler {
	return &ClinicalIngestionHandler{service: s, maxAudioBytes: max, transcriptionHash: trHash, analysisHash: analysisHash}
}
func (h *ClinicalIngestionHandler) WithHealth(f func(context.Context) map[string]any) *ClinicalIngestionHandler {
	h.health = f
	return h
}
func (h *ClinicalIngestionHandler) Health(c echo.Context) error {
	if h.health == nil {
		return c.JSON(200, map[string]any{"worker": "not_configured"})
	}
	return c.JSON(200, h.health(c.Request().Context()))
}
func (h *ClinicalIngestionHandler) Upload(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	format := ""
	switch c.Request().Header.Get("Content-Type") {
	case "audio/wav":
		format = "wav"
	case "audio/webm":
		format = "webm"
	case "audio/ogg":
		format = "ogg"
	case "audio/mp4":
		format = "mp4"
	case "audio/x-m4a":
		format = "m4a"
	default:
		return writeAPIError(c, 415, "unsupported_media_type", "unsupported audio media type")
	}
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, h.maxAudioBytes)
	out, err := h.service.Upload(c.Request().Context(), t, id, p.UserID, format, c.Request().Body)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(201, out)
}
func (h *ClinicalIngestionHandler) Artifacts(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	out, err := h.service.ListArtifacts(c.Request().Context(), t, id, p.UserID)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(200, map[string]any{"items": out})
}
func (h *ClinicalIngestionHandler) Transcripts(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	out, err := h.service.ListTranscripts(c.Request().Context(), t, id, p.UserID)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(200, map[string]any{"items": out})
}
func (h *ClinicalIngestionHandler) Transcript(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	out, err := h.service.GetTranscript(c.Request().Context(), t, id, p.UserID)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(200, out)
}
func (h *ClinicalIngestionHandler) DeleteArtifact(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	out, err := h.service.DeleteArtifact(c.Request().Context(), t, id, p.UserID)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(200, out)
}
func decodeIngestionCommand(c echo.Context, out any, max int64) error {
	d := json.NewDecoder(http.MaxBytesReader(c.Response(), c.Request().Body, max))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		return echo.NewHTTPError(400, "invalid ingestion command")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return echo.NewHTTPError(400, "invalid ingestion command")
	}
	return nil
}
func (h *ClinicalIngestionHandler) Transcribe(c echo.Context) error {
	return h.enqueue(c, ingestion.Transcribe)
}
func (h *ClinicalIngestionHandler) Analyze(c echo.Context) error {
	return h.enqueue(c, ingestion.Analyze)
}
func (h *ClinicalIngestionHandler) enqueue(c echo.Context, kind string) error {
	t, p, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	var source uuid.UUID
	hash := h.transcriptionHash
	if kind == ingestion.Transcribe {
		var body struct {
			ID uuid.UUID `json:"artifact_id"`
		}
		if err := decodeIngestionCommand(c, &body, 4096); err != nil {
			return err
		}
		source = body.ID
	} else {
		var body struct {
			ID uuid.UUID `json:"transcript_version_id"`
		}
		if err := decodeIngestionCommand(c, &body, 4096); err != nil {
			return err
		}
		source = body.ID
		hash = h.analysisHash
	}
	if !ingestion.ValidHash(hash) {
		return writeAPIError(c, 503, "unavailable", "processing configuration unavailable")
	}
	out, created, err := h.service.Enqueue(c.Request().Context(), t, id, p.UserID, source, kind, hash)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	code := 200
	if created {
		code = 202
	}
	return c.JSON(code, out)
}
func (h *ClinicalIngestionHandler) CorrectTranscript(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	var body struct {
		Text     string              `json:"text"`
		Segments []ingestion.Segment `json:"segments"`
	}
	if err := decodeIngestionCommand(c, &body, 2*1024*1024); err != nil {
		return err
	}
	out, err := h.service.CorrectTranscript(c.Request().Context(), t, id, p.UserID, body.Text, body.Segments)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(201, out)
}
func (h *ClinicalIngestionHandler) Job(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	out, err := h.service.GetJob(c.Request().Context(), t, id, p.UserID)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(200, out)
}
func (h *ClinicalIngestionHandler) Cancel(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	out, err := h.service.CancelJob(c.Request().Context(), t, id, p.UserID)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(200, out)
}
func (h *ClinicalIngestionHandler) Retry(c echo.Context) error {
	t, p, id, err := clinicalSessionContext(c, "id")
	if err != nil {
		return err
	}
	out, err := h.service.RetryJob(c.Request().Context(), t, id, p.UserID)
	if err != nil {
		return handleLongitudinalError(c, err)
	}
	return c.JSON(202, out)
}
