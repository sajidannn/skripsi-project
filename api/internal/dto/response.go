// Package dto defines HTTP-layer request and response structs.
//
// Separation of concerns:
//   - dto    → HTTP shapes (binding/validation tags, response envelopes)
//   - model  → domain/DB structs (no HTTP concerns)
//   - service → accepts dto requests, returns model entities
//   - handler → maps model → dto response before writing JSON
package dto

// Response is the unified success envelope for all API responses.
type Response[T any] struct {
	Status string `json:"status"` // always "success"
	Data   T      `json:"data"`
}

// Success wraps data in a success envelope.
func Success[T any](data T) Response[T] {
	return Response[T]{Status: "success", Data: data}
}
