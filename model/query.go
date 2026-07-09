package model

// Query is the top-level input to Generator.Scopes: a global search term
// plus a SelectParameter describing filters, sorting, pagination, field
// projection, and preloads.
//
// Query values are usually produced by decoding a request — see the
// binding package for a URL-query-string parser that populates a Query
// from a request such as:
//
//	GET /v2/products?search=capital&sort=type_id:asc&pageSize=10&page=2&filter=type_id:1:equals
type Query struct {
	// Search is a single free-text term matched, via LIKE/ILIKE, against
	// every field marked Searchable in the target ModelMeta. Empty means
	// no global search is applied.
	Search string `json:"search"`

	// Filter and Sort are legacy raw string fields retained for JSON
	// compatibility with older callers. Generator does not read them —
	// it only consults SelectParameter.Filters, SelectParameter.Sorts,
	// and SelectParameter.FilterGroups. New callers should populate
	// SelectParameter directly (or use the binding package, which parses
	// URL query filter/sort syntax straight into SelectParameter).
	Filter string `json:"filter"`
	Sort   string `json:"sort"`

	// SelectParameter carries the structured filter, sort, pagination,
	// field-projection, and preload data that Generator.Scopes actually
	// compiles.
	SelectParameter SelectParameter
}

// SelectParameter groups every structured input Generator.Scopes compiles
// into GORM scopes, aside from the global Query.Search term.
type SelectParameter struct {
	// Filters is a flat list of comparisons, implicitly ANDed together.
	// Every FieldName must be declared in the target ModelMeta.Fields and
	// every Operator must be whitelisted for that field, or Scopes
	// returns an error.
	Filters []Filter `json:"filters"`

	// FilterGroups is a list of independently-parenthesized filter
	// groups, each with its own AND/OR Condition and optional nested
	// sub-groups. Every top-level FilterGroup is ANDed onto the query
	// alongside Filters.
	FilterGroups []FilterGroup `json:"filter_groups"`

	// Sorts is an ordered list of fields to sort by. When empty,
	// Generator falls back to ordering by its configured
	// DefaultFieldForSort. A Sort referencing a field not declared in
	// the schema is silently skipped rather than treated as an error.
	Sorts []Sort `json:"sorts"`

	// PageDescriptor controls LIMIT/OFFSET pagination. See Pagination
	// for default values when Page or PageSize is unset.
	PageDescriptor Pagination `json:"page_descriptor"`

	// Fields, when non-empty, restricts the query to SELECT only these
	// columns instead of SELECT *. Each entry must be a query-facing
	// field name declared in the schema; unknown names are silently
	// skipped.
	Fields []string `json:"fields"`

	// Preloads lists GORM association names to eager-load via
	// db.Preload. Names are passed through to GORM as-is and are not
	// validated against the schema.
	Preloads []string `json:"preloads"`
}
