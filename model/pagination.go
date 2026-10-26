package model

// Pagination describes offset-based (LIMIT/OFFSET) pagination. Page is
// 1-indexed. When Page is less than 1 it defaults to 1; when PageSize is
// less than 1 it defaults to 20. There is no enforced upper bound on
// PageSize in this struct. Generator.MaxPageSize and MaxOffset can bound
// execution; StrictQueries rejects negative or excessive requests instead
// of silently defaulting or clamping them.
type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}
