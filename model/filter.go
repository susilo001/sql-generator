package model

// Filter is a single comparison to apply to one field: FieldName Operator
// Value (or RangeValues, for operators that take more than one value).
//
// Filter is used both as a flat, top-level entry in
// SelectParameter.Filters (all of which are implicitly ANDed together) and
// as a leaf inside a FilterGroup, where its result is combined with
// siblings according to the group's Condition.
type Filter struct {
	// FieldName is the query-facing field name, matched against a key in
	// ModelMeta.Fields. It is not a raw SQL column name.
	FieldName string `json:"field_name"`

	// Operator selects the comparison to perform. It must be whitelisted
	// for FieldName in the target ModelMeta's FieldMeta.Operators.
	Operator Operator `json:"operator"`

	// Condition is currently unused by Generator: a top-level Filter's
	// boolean joiner is always AND (see SelectParameter.Filters), and a
	// Filter nested inside a FilterGroup takes its joiner from the
	// enclosing FilterGroup.Condition instead of from this field. It is
	// kept for JSON compatibility with clients that may set it, and is
	// safe to leave zero-valued.
	Condition Condition `json:"condition"`

	// Value is the comparand for single-value operators (IsEqual,
	// IsNotEqual, IsLessThan, IsMoreThan, IsLessThanOrEqual,
	// IsMoreThanOrEqual, IsContain, IsBeginWith, IsEndWith). Ignored by
	// RangeValues-based operators and by IsNull/IsNotNull.
	Value any `json:"value"`

	// RangeValues holds the comparands for multi-value operators.
	// IsBetween requires exactly two elements (low, high); IsIn and
	// IsNotIn accept any number of elements. Ignored by all other
	// operators.
	RangeValues []any `json:"range_values"`
}

// FilterGroup is a set of Filters and nested FilterGroups combined by a
// single Condition (AND/OR), allowing arbitrarily deep boolean grouping —
// e.g. "(role = 'admin' OR role = 'owner') AND status = 'active'".
//
// Generator compiles a FilterGroup into one fully parenthesized SQL
// fragment, folding nested FilterGroups into the same fragment so their own
// Condition is preserved regardless of how deep they are nested.
type FilterGroup struct {
	// Condition is the boolean joiner applied between every sibling entry
	// in Filters and FilterGroups within this group: And or Or.
	Condition Condition `json:"condition"`

	// Filters are the leaf comparisons directly in this group.
	Filters []Filter `json:"filters"`

	// FilterGroups are nested sub-groups, each compiled to its own
	// parenthesized fragment before being combined into this group via
	// Condition.
	FilterGroups []FilterGroup `json:"filter_groups"`
}
