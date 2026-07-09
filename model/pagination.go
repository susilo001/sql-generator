package model

// Pagination describes offset-based (LIMIT/OFFSET) pagination. Page is
// 1-indexed. When Page is less than 1 it defaults to 1; when PageSize is
// less than 1 it defaults to 20. There is no enforced upper bound on
// PageSize — callers that need to cap it should validate PageSize before
// building a Query.
type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}
