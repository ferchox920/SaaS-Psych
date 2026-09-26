package handlers

import (
	_ "embed"
	"net/http"

	"github.com/labstack/echo/v4"
)

//go:embed openapi.yaml
var embeddedOpenAPISpec []byte

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
	return c.Blob(http.StatusOK, "application/yaml; charset=utf-8", embeddedOpenAPISpec)
}
