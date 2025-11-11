package types

import "errors"

const (
	// NotFoundCode is code for the error of no object found
	NotFoundCode = "NOT_FOUND"
	// UnAuthorizedCode ...
	UnAuthorizedCode = "UNAUTHORIZED"
	// ForbiddenCode ...
	ForbiddenCode = "FORBIDDEN"
	// RateLimitCode
	RateLimitCode = "TOO_MANY_REQUEST"
	// GeneralCode ...
	GeneralCode = "UNKNOWN"
)

var NotFoundCodeErr = errors.New(NotFoundCode)
