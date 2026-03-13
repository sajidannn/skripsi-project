package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sajidannn/pos-api/internal/apierr"
)

// errorResponse is the unified JSON shape for all error responses.
type errorResponse struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Errors  map[string]string `json:"errors,omitempty"` // populated for 422 validation errors
	Detail  string            `json:"detail,omitempty"` // populated for 5xx in debug mode only
}

// ErrorHandler is a Gin middleware that must be registered FIRST (outermost)
// so it runs last on the way out.
//
// Handlers should NOT call c.JSON for errors. Instead they call:
//
//	_ = c.Error(apierr.Wrap(err, "thing not found"))
//	return
//
// This middleware collects those errors and renders a single, consistent JSON
// response. Production mode (debug=false) never leaks internal details for 5xx.
func ErrorHandler(debug bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next() // run handlers first

		if len(c.Errors) == 0 {
			return // no errors — nothing to do
		}

		// Use the last error attached to the context.
		last := c.Errors.Last()

		var appErr *apierr.AppError

		// Try to cast directly to *apierr.AppError.
		if ae, ok := last.Err.(*apierr.AppError); ok {
			appErr = ae
		} else {
			// Unknown error type — treat as 500.
			appErr = apierr.Internal(last.Err, last.Err.Error())
		}

		resp := errorResponse{
			Code:    appErr.Code,
			Message: appErr.Message,
			Errors:  appErr.Fields, // nil for non-validation errors → omitted from JSON
		}

		// Only expose internal detail for 5xx AND only when debug mode is on.
		if debug && appErr.HTTPStatus >= http.StatusInternalServerError && appErr.Detail != "" {
			resp.Detail = appErr.Detail
		}

		c.JSON(appErr.HTTPStatus, resp)
	}
}
