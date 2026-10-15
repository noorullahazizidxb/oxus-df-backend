// File: internal/products/handler.go
// Usage: HTTP handlers for the products CRUD and export endpoints, plus API <-> domain mapping.

package products

import (
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"oxus-df-backend/internal/apperr"
	"oxus-df-backend/internal/auth"
	"oxus-df-backend/internal/httpapi/v1/gen"
	"oxus-df-backend/internal/platform/httpx"
)

const (
	defaultPage     int32 = 1
	defaultPageSize int32 = 20
	xlsxContentType       = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
)

// Strict patterns are checked before parsing so hostile input such as "1e999999"
// never reaches the decimal library.
var (
	moneyPattern = regexp.MustCompile(`^\d{1,16}(\.\d{1,2})?$`)
	ratePattern  = regexp.MustCompile(`^\d{1,3}(\.\d{1,2})?$`)
)

// Handler reads the request, calls the service, writes the response.
type Handler struct {
	svc *Service
}

// NewHandler creates the products HTTP handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// ListProducts implements GET /admin/products.
func (h *Handler) ListProducts(c *gin.Context, params gen.ListProductsParams) {
	q := ListQuery{
		Search:   httpx.ValueOr(params.Q, ""),
		Page:     httpx.ValueOr(params.Page, defaultPage),
		PageSize: httpx.ValueOr(params.PageSize, defaultPageSize),
	}
	if params.Status != nil {
		s := Status(*params.Status)
		q.Status = &s
	}

	res, err := h.svc.List(c.Request.Context(), q)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	data := make([]gen.Product, 0, len(res.Items))
	for _, p := range res.Items {
		data = append(data, toAPI(p))
	}
	c.JSON(http.StatusOK, gen.ProductList{
		Data: data,
		Meta: gen.PageMeta{Page: res.Page, PageSize: res.PageSize, Total: res.Total},
	})
}

// CreateProduct implements POST /admin/products.
func (h *Handler) CreateProduct(c *gin.Context) {
	actor, err := auth.PrincipalFromGin(c)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	var req gen.CreateProductRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	in, err := toCreateInput(req)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	p, err := h.svc.Create(c.Request.Context(), actor, in)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.Header("Location", "/api/v1/admin/products/"+p.ID.String())
	c.JSON(http.StatusCreated, toAPI(p))
}

// GetProduct implements GET /admin/products/{id}.
func (h *Handler) GetProduct(c *gin.Context, id uuid.UUID) {
	p, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAPI(p))
}

// UpdateProduct implements PUT /admin/products/{id}.
func (h *Handler) UpdateProduct(c *gin.Context, id uuid.UUID) {
	actor, err := auth.PrincipalFromGin(c)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	var req gen.UpdateProductRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	in, err := toUpdateInput(req)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	p, err := h.svc.Update(c.Request.Context(), actor, id, in)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAPI(p))
}

// DeleteProduct implements DELETE /admin/products/{id}.
func (h *Handler) DeleteProduct(c *gin.Context, id uuid.UUID) {
	actor, err := auth.PrincipalFromGin(c)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	if err := h.svc.Delete(c.Request.Context(), actor, id); err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ExportProducts implements GET /admin/exports/products.
func (h *Handler) ExportProducts(c *gin.Context) {
	actor, err := auth.PrincipalFromGin(c)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	data, err := h.svc.Export(c.Request.Context(), actor)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="products.xlsx"`)
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, xlsxContentType, data)
}

// ---- mapping between API types and domain types ----

// toAPI converts a domain Product to the API response shape (decimals become strings).
func toAPI(p Product) gen.Product {
	return gen.Product{
		Id:          p.ID,
		Code:        p.Code,
		Name:        p.Name,
		Description: p.Description,
		Currency:    p.Currency,
		MinAmount:   p.MinAmount.StringFixed(2),
		MaxAmount:   p.MaxAmount.StringFixed(2),
		AnnualRate:  p.AnnualRate.StringFixed(2),
		Status:      gen.ProductStatus(p.Status),
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

// toCreateInput converts the API create request to a domain input, parsing the decimal strings safely.
func toCreateInput(req gen.CreateProductRequest) (CreateInput, error) {
	var d decimals
	in := CreateInput{
		Code:       req.Code,
		Name:       req.Name,
		Currency:   req.Currency,
		MinAmount:  d.parse("min_amount", req.MinAmount, moneyPattern),
		MaxAmount:  d.parse("max_amount", req.MaxAmount, moneyPattern),
		AnnualRate: d.parse("annual_rate", req.AnnualRate, ratePattern),
	}
	if req.Description != nil {
		in.Description = *req.Description
	}
	if req.Status != nil {
		in.Status = Status(*req.Status)
	}
	return in, d.err()
}

// toUpdateInput converts the API update request to a domain input, parsing the decimal strings safely.
func toUpdateInput(req gen.UpdateProductRequest) (UpdateInput, error) {
	var d decimals
	in := UpdateInput{
		Name:        req.Name,
		Description: req.Description,
		MinAmount:   d.parse("min_amount", req.MinAmount, moneyPattern),
		MaxAmount:   d.parse("max_amount", req.MaxAmount, moneyPattern),
		AnnualRate:  d.parse("annual_rate", req.AnnualRate, ratePattern),
		Status:      Status(req.Status),
	}
	return in, d.err()
}

// decimals collects number-format errors so the client sees all of them at once.
type decimals struct {
	fields []apperr.Field
}

// parse checks the text with a strict pattern, converts it to a decimal and remembers any error.
func (d *decimals) parse(name, raw string, pattern *regexp.Regexp) decimal.Decimal {
	if pattern.MatchString(raw) {
		if v, err := decimal.NewFromString(raw); err == nil {
			return v
		}
	}
	d.fields = append(d.fields, apperr.Field{Name: name, Message: "must be a decimal number with at most 2 decimal places"})
	return decimal.Zero
}

// err returns all remembered number errors as one validation error, or nil.
func (d *decimals) err() error {
	if len(d.fields) == 0 {
		return nil
	}
	return apperr.Validation("validation failed", d.fields...)
}
