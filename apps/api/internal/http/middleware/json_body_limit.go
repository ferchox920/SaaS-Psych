package middleware

import (
	"bytes"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	httpresponse "sessionflow/apps/api/internal/http/response"
)

func JSONBodyLimit(maxBytes int64) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			request := c.Request()
			mediaType, _, _ := mime.ParseMediaType(request.Header.Get(echo.HeaderContentType))
			if !strings.EqualFold(mediaType, echo.MIMEApplicationJSON) || request.Body == nil {
				return next(c)
			}
			if request.ContentLength > maxBytes {
				return httpresponse.WriteError(c, http.StatusRequestEntityTooLarge, "payload_too_large", "JSON body exceeds the request limit")
			}
			body, err := io.ReadAll(io.LimitReader(request.Body, maxBytes+1))
			if err != nil {
				return httpresponse.WriteError(c, http.StatusBadRequest, "validation_error", "invalid request body")
			}
			if int64(len(body)) > maxBytes {
				return httpresponse.WriteError(c, http.StatusRequestEntityTooLarge, "payload_too_large", "JSON body exceeds the request limit")
			}
			request.Body = io.NopCloser(bytes.NewReader(body))
			request.ContentLength = int64(len(body))
			return next(c)
		}
	}
}
