package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/labstack/echo/v4"
)

const swaggerUIHTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>SessionFlow API Docs</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  </head>
  <body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script>
      window.ui = SwaggerUIBundle({
        url: '/docs/openapi.yaml',
        dom_id: '#swagger-ui',
        deepLinking: true
      });
    </script>
  </body>
</html>`

func DocsUI(c echo.Context) error {
	return c.HTML(http.StatusOK, swaggerUIHTML)
}

func OpenAPISpec(c echo.Context) error {
	specPath, err := resolveOpenAPISpecPath()
	if err != nil {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "openapi spec path resolution failed")
	}

	content, err := os.ReadFile(specPath)
	if err != nil {
		return writeAPIError(c, http.StatusInternalServerError, "internal_error", "openapi spec unavailable")
	}

	return c.Blob(http.StatusOK, "application/yaml; charset=utf-8", content)
}

func resolveOpenAPISpecPath() (string, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", os.ErrNotExist
	}

	handlersDir := filepath.Dir(currentFile)
	return filepath.Clean(filepath.Join(handlersDir, "..", "..", "..", "..", "..", "docs", "openapi.yaml")), nil
}
