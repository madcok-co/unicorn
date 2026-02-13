// Package pagination provides pagination helpers for Unicorn Framework.
//
// Supports:
//   - Offset-based pagination (page/limit)
//   - Cursor-based pagination (next/prev)
//   - Keyset pagination (efficient for large datasets)
//
// Usage:
//
//	import (
//	    "github.com/madcok-co/unicorn/contrib/pagination"
//	)
//
//	// Offset pagination
//	params := pagination.ParseOffsetParams(req)
//	result, err := pagination.OffsetPaginate(ctx.DB(), &users, params)
//
//	// Cursor pagination
//	params := pagination.ParseCursorParams(req)
//	result, err := pagination.CursorPaginate(ctx.DB(), &users, "id", params)
package pagination

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

// OffsetParams represents offset-based pagination parameters
type OffsetParams struct {
	Page     int    // Current page number (1-based)
	Limit    int    // Items per page
	Sort     string // Sort field
	Order    string // Sort order (asc/desc)
	MaxLimit int    // Maximum allowed limit
}

// CursorParams represents cursor-based pagination parameters
type CursorParams struct {
	Cursor   string // Cursor for next page
	Limit    int    // Items per page
	Sort     string // Sort field
	Order    string // Sort order (asc/desc)
	MaxLimit int    // Maximum allowed limit
}

// OffsetResult represents offset pagination result
type OffsetResult struct {
	Data        interface{} `json:"data"`
	Total       int64       `json:"total"`
	Page        int         `json:"page"`
	Limit       int         `json:"limit"`
	TotalPages  int         `json:"total_pages"`
	HasPrevious bool        `json:"has_previous"`
	HasNext     bool        `json:"has_next"`
}

// CursorResult represents cursor pagination result
type CursorResult struct {
	Data       interface{} `json:"data"`
	NextCursor string      `json:"next_cursor,omitempty"`
	PrevCursor string      `json:"prev_cursor,omitempty"`
	HasNext    bool        `json:"has_next"`
	HasPrev    bool        `json:"has_prev"`
}

// DefaultOffsetParams returns default offset parameters
func DefaultOffsetParams() *OffsetParams {
	return &OffsetParams{
		Page:     1,
		Limit:    20,
		Order:    "asc",
		MaxLimit: 100,
	}
}

// DefaultCursorParams returns default cursor parameters
func DefaultCursorParams() *CursorParams {
	return &CursorParams{
		Limit:    20,
		Order:    "asc",
		MaxLimit: 100,
	}
}

// ParseOffsetParams parses offset pagination from query params
func ParseOffsetParams(page, limit int, sort, order string) *OffsetParams {
	params := DefaultOffsetParams()

	if page > 0 {
		params.Page = page
	}

	if limit > 0 {
		params.Limit = limit
		if limit > params.MaxLimit {
			params.Limit = params.MaxLimit
		}
	}

	if sort != "" {
		params.Sort = sort
	}

	if order != "" {
		order = strings.ToLower(order)
		if order == "asc" || order == "desc" {
			params.Order = order
		}
	}

	return params
}

// ParseCursorParams parses cursor pagination from query params
func ParseCursorParams(cursor string, limit int, sort, order string) *CursorParams {
	params := DefaultCursorParams()

	if cursor != "" {
		params.Cursor = cursor
	}

	if limit > 0 {
		params.Limit = limit
		if limit > params.MaxLimit {
			params.Limit = params.MaxLimit
		}
	}

	if sort != "" {
		params.Sort = sort
	}

	if order != "" {
		order = strings.ToLower(order)
		if order == "asc" || order == "desc" {
			params.Order = order
		}
	}

	return params
}

// Offset calculates the offset for SQL OFFSET clause
func (p *OffsetParams) Offset() int {
	return (p.Page - 1) * p.Limit
}

// Validate validates offset parameters
func (p *OffsetParams) Validate() error {
	if p.Page < 1 {
		return fmt.Errorf("page must be >= 1")
	}
	if p.Limit < 1 {
		return fmt.Errorf("limit must be >= 1")
	}
	if p.Limit > p.MaxLimit {
		return fmt.Errorf("limit cannot exceed %d", p.MaxLimit)
	}
	if p.Order != "asc" && p.Order != "desc" {
		return fmt.Errorf("order must be 'asc' or 'desc'")
	}
	return nil
}

// Validate validates cursor parameters
func (p *CursorParams) Validate() error {
	if p.Limit < 1 {
		return fmt.Errorf("limit must be >= 1")
	}
	if p.Limit > p.MaxLimit {
		return fmt.Errorf("limit cannot exceed %d", p.MaxLimit)
	}
	if p.Order != "asc" && p.Order != "desc" {
		return fmt.Errorf("order must be 'asc' or 'desc'")
	}
	return nil
}

// EncodeCursor encodes a cursor value
func EncodeCursor(value string) string {
	return base64.URLEncoding.EncodeToString([]byte(value))
}

// DecodeCursor decodes a cursor value
func DecodeCursor(cursor string) (string, error) {
	decoded, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return "", fmt.Errorf("invalid cursor: %w", err)
	}
	return string(decoded), nil
}

// NewOffsetResult creates a new offset pagination result
func NewOffsetResult(data interface{}, total int64, params *OffsetParams) *OffsetResult {
	totalPages := int((total + int64(params.Limit) - 1) / int64(params.Limit))
	if totalPages < 1 {
		totalPages = 1
	}

	return &OffsetResult{
		Data:        data,
		Total:       total,
		Page:        params.Page,
		Limit:       params.Limit,
		TotalPages:  totalPages,
		HasPrevious: params.Page > 1,
		HasNext:     params.Page < totalPages,
	}
}

// NewCursorResult creates a new cursor pagination result
func NewCursorResult(data interface{}, nextCursor, prevCursor string, hasNext, hasPrev bool) *CursorResult {
	return &CursorResult{
		Data:       data,
		NextCursor: nextCursor,
		PrevCursor: prevCursor,
		HasNext:    hasNext,
		HasPrev:    hasPrev,
	}
}

// BuildOrderByClause builds SQL ORDER BY clause
func BuildOrderByClause(sort, order string) string {
	if sort == "" {
		return ""
	}

	// Sanitize sort field (prevent SQL injection)
	sort = sanitizeField(sort)

	if order == "" {
		order = "asc"
	}

	return fmt.Sprintf("%s %s", sort, strings.ToUpper(order))
}

// sanitizeField sanitizes field name to prevent SQL injection
func sanitizeField(field string) string {
	// Only allow alphanumeric characters, underscores, and dots
	var result strings.Builder
	for _, char := range field {
		if (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '_' || char == '.' {
			result.WriteRune(char)
		}
	}
	return result.String()
}

// PageInfo represents pagination metadata
type PageInfo struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// CursorInfo represents cursor pagination metadata
type CursorInfo struct {
	NextCursor string `json:"next_cursor,omitempty"`
	PrevCursor string `json:"prev_cursor,omitempty"`
	HasNext    bool   `json:"has_next"`
	HasPrev    bool   `json:"has_prev"`
}

// Links represents pagination links (for HATEOAS)
type Links struct {
	Self  string `json:"self,omitempty"`
	First string `json:"first,omitempty"`
	Prev  string `json:"prev,omitempty"`
	Next  string `json:"next,omitempty"`
	Last  string `json:"last,omitempty"`
}

// BuildLinks builds pagination links
func BuildLinks(baseURL string, page, totalPages int) *Links {
	links := &Links{
		Self:  fmt.Sprintf("%s?page=%d", baseURL, page),
		First: fmt.Sprintf("%s?page=1", baseURL),
		Last:  fmt.Sprintf("%s?page=%d", baseURL, totalPages),
	}

	if page > 1 {
		links.Prev = fmt.Sprintf("%s?page=%d", baseURL, page-1)
	}

	if page < totalPages {
		links.Next = fmt.Sprintf("%s?page=%d", baseURL, page+1)
	}

	return links
}

// BuildCursorLinks builds cursor-based pagination links
func BuildCursorLinks(baseURL, nextCursor, prevCursor string) *Links {
	links := &Links{
		Self: baseURL,
	}

	if nextCursor != "" {
		links.Next = fmt.Sprintf("%s?cursor=%s", baseURL, nextCursor)
	}

	if prevCursor != "" {
		links.Prev = fmt.Sprintf("%s?cursor=%s", baseURL, prevCursor)
	}

	return links
}

// ParsePageFromQuery parses page number from string
func ParsePageFromQuery(pageStr string) int {
	if pageStr == "" {
		return 1
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		return 1
	}
	return page
}

// ParseLimitFromQuery parses limit from string
func ParseLimitFromQuery(limitStr string, defaultLimit, maxLimit int) int {
	if limitStr == "" {
		return defaultLimit
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

// CursorEncoder interface for custom cursor encoding
type CursorEncoder interface {
	Encode(value interface{}) string
	Decode(cursor string) (interface{}, error)
}

// DefaultCursorEncoder uses base64 encoding
type DefaultCursorEncoder struct{}

// Encode encodes value to cursor
func (e *DefaultCursorEncoder) Encode(value interface{}) string {
	str := fmt.Sprintf("%v", value)
	return EncodeCursor(str)
}

// Decode decodes cursor to value
func (e *DefaultCursorEncoder) Decode(cursor string) (interface{}, error) {
	return DecodeCursor(cursor)
}
