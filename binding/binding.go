package binding

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/susilo001/sql-generator/model"
)

// ParseBody decodes a JSON request body from r into a model.Query.
// Works with any framework that exposes the body as an io.Reader (net/http, Gin, etc.).
//
// Example — net/http:
//
//	q, err := binding.ParseBody(r.Body)
//
// Example — Gin:
//
//	q, err := binding.ParseBody(c.Request.Body)
func ParseBody(r io.Reader) (model.Query, error) {
	var q model.Query
	if err := json.NewDecoder(r).Decode(&q); err != nil {
		return model.Query{}, fmt.Errorf("binding: failed to decode query: %w", err)
	}
	return q, nil
}

// ParseBytes decodes a JSON byte slice into a model.Query.
// Use this with frameworks that expose the body as []byte (Fiber, Echo with c.Body(), etc.).
//
// Example — Fiber:
//
//	q, err := binding.ParseBytes(c.Body())
//
// Example — Echo:
//
//	q, err := binding.ParseBytes(c.Request().Body) // or read body bytes first
func ParseBytes(b []byte) (model.Query, error) {
	var q model.Query
	if err := json.Unmarshal(b, &q); err != nil {
		return model.Query{}, fmt.Errorf("binding: failed to decode query: %w", err)
	}
	return q, nil
}
