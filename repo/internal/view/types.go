package view

import (
	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/google/uuid"
)

// PageCtx carries per-request context passed to every page template.
type PageCtx struct {
	User  NavUser
	Flash *Flash
}

// NavUser is the minimal user info needed for layout/nav.
type NavUser struct {
	ID       uuid.UUID
	Username string
	Role     domain.UserRole
}

// Flash is a one-time feedback message.
type Flash struct {
	Type    string // "success" | "error" | "warning" | "queued"
	Message string
}

// Pagination carries computed values for the pagination component.
type Pagination struct {
	Page       int
	PageSize   int
	Total      int
	TotalPages int
	BasePath   string // e.g. "/tiers"
	QueryStr   string // extra query params without leading &, e.g. "q=foo"
}

func NewPagination(page, pageSize, total int, basePath, queryStr string) Pagination {
	totalPages := total / pageSize
	if total%pageSize != 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}
	return Pagination{
		Page: page, PageSize: pageSize, Total: total,
		TotalPages: totalPages, BasePath: basePath, QueryStr: queryStr,
	}
}
