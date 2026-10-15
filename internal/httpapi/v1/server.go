// File: internal/httpapi/v1/server.go
// Usage: Connects the generated API interface to the module handlers. No business logic here.

// Package v1 wires the generated OpenAPI server interface to the module
// handlers. It contains no business logic.
package v1

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"oxus-df-backend/internal/auth"
	"oxus-df-backend/internal/httpapi/v1/gen"
	"oxus-df-backend/internal/products"
)

// Server implements gen.ServerInterface by delegating to module handlers.
type Server struct {
	auth          *auth.Handler
	products      *products.Handler
	minAppVersion string
}

var _ gen.ServerInterface = (*Server)(nil)

// Login forwards POST /admin/auth/login to the auth handler.
func (s *Server) Login(c *gin.Context) { s.auth.Login(c) }

// ListProducts forwards GET /admin/products to the products handler.
func (s *Server) ListProducts(c *gin.Context, params gen.ListProductsParams) {
	s.products.ListProducts(c, params)
}

// CreateProduct forwards POST /admin/products to the products handler.
func (s *Server) CreateProduct(c *gin.Context) { s.products.CreateProduct(c) }

// GetProduct forwards GET /admin/products/{id} to the products handler.
func (s *Server) GetProduct(c *gin.Context, id uuid.UUID) { s.products.GetProduct(c, id) }

// UpdateProduct forwards PUT /admin/products/{id} to the products handler.
func (s *Server) UpdateProduct(c *gin.Context, id uuid.UUID) { s.products.UpdateProduct(c, id) }

// DeleteProduct forwards DELETE /admin/products/{id} to the products handler.
func (s *Server) DeleteProduct(c *gin.Context, id uuid.UUID) { s.products.DeleteProduct(c, id) }

// ExportProducts forwards GET /admin/exports/products to the products handler.
func (s *Server) ExportProducts(c *gin.Context) { s.products.ExportProducts(c) }

// GetAppConfig lets the mobile app learn the minimum supported version and
// force an update when it is older.
func (s *Server) GetAppConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gen.AppConfig{
		MinAppVersion: s.minAppVersion,
		ServerTime:    time.Now().UTC(),
	})
}
