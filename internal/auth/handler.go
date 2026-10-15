// File: internal/auth/handler.go
// Usage: HTTP handler for the login endpoint. Reads the request and writes the response only.

package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"oxus-df-backend/internal/httpapi/v1/gen"
	"oxus-df-backend/internal/platform/httpx"
)

// Handler reads the request, calls the service, writes the response.
// Business rules live in service.go.
type Handler struct {
	svc *Service
}

// NewHandler creates the auth HTTP handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Login implements POST /admin/auth/login.
func (h *Handler) Login(c *gin.Context) {
	var req gen.LoginRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	res, err := h.svc.Login(c.Request.Context(), LoginInput{Email: string(req.Email), Password: req.Password})
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gen.LoginResponse{
		AccessToken: res.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(res.ExpiresIn.Seconds()),
	})
}
