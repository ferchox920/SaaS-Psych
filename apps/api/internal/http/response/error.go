package response

import "github.com/labstack/echo/v4"

type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func WriteError(c echo.Context, status int, code, message string, details ...any) error {
	var payloadDetails any
	if len(details) > 0 {
		payloadDetails = details[0]
	}

	return c.JSON(status, ErrorEnvelope{
		Error: ErrorBody{
			Code:    code,
			Message: message,
			Details: payloadDetails,
		},
	})
}
