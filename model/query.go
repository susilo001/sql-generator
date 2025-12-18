package model

type Query struct {
	Search          string
	Filter          string
	Sort            string
	SelectParameter SelectParameter
	IncludeDeleted  bool // Include soft-deleted records
	Distinct        bool // Apply DISTINCT to query
}

type SelectParameter struct {
	Filters        []Filter      // Simple filters (deprecated, use FilterGroups)
	FilterGroups   []FilterGroup // Complex filter groups with AND/OR logic
	Sorts          []Sort
	PageDescriptor Pagination
	Fields         []string // Field projection (SELECT specific columns)
	Preloads       []string // Relationships to preload
}
