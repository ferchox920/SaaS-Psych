package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	httpresponse "sessionflow/apps/api/internal/http/response"
	authusecase "sessionflow/apps/api/internal/usecase/auth"
)

type principalContextKey struct{}

type Principal struct {
	UserID uuid.UUID
	Role   string
}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}

func RequireAuth(accessSecret string) echo.MiddlewareFunc {
	secret := []byte(accessSecret)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := strings.TrimSpace(c.Request().Header.Get("Authorization"))
			if authHeader == "" {
				return httpresponse.WriteError(c, http.StatusUnauthorized, "unauthorized", "missing authorization header", map[string]any{
					"field":  "Authorization",
					"reason": "required",
				})
			}

			tokenString, ok := strings.CutPrefix(authHeader, "Bearer ")
			if !ok || strings.TrimSpace(tokenString) == "" {
				return httpresponse.WriteError(c, http.StatusUnauthorized, "unauthorized", "invalid authorization header", map[string]any{
					"field":  "Authorization",
					"reason": "invalid_bearer_format",
				})
			}

			claims := &authusecase.AccessClaims{}
			token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrTokenSignatureInvalid
				}
				return secret, nil
			})
			if err != nil || !token.Valid {
				return httpresponse.WriteError(c, http.StatusUnauthorized, "unauthorized", "invalid access token", map[string]any{
					"field":  "Authorization",
					"reason": "invalid_or_expired_token",
				})
			}

			userID, err := uuid.Parse(claims.Subject)
			if err != nil {
				return httpresponse.WriteError(c, http.StatusUnauthorized, "unauthorized", "invalid access token subject")
			}

			tenantID, err := uuid.Parse(claims.TenantID)
			if err != nil {
				return httpresponse.WriteError(c, http.StatusUnauthorized, "unauthorized", "invalid access token tenant")
			}

			contextTenantID, ok := TenantIDFromContext(c.Request().Context())
			if !ok || contextTenantID != tenantID {
				return httpresponse.WriteError(c, http.StatusUnauthorized, "unauthorized", "tenant mismatch between header and token")
			}

			ctx := WithPrincipal(c.Request().Context(), Principal{
				UserID: userID,
				Role:   claims.Role,
			})
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}

func RequireRole(roles ...string) echo.MiddlewareFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			principal, ok := PrincipalFromContext(c.Request().Context())
			if !ok {
				return httpresponse.WriteError(c, http.StatusUnauthorized, "unauthorized", "auth context missing")
			}

			if _, exists := allowed[principal.Role]; !exists {
				return httpresponse.WriteError(c, http.StatusForbidden, "forbidden", "insufficient role")
			}

			return next(c)
		}
	}
}
