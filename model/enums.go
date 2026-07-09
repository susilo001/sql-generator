// Package model defines the data types that describe a query: filters,
// filter groups, sorting, pagination, and field/preload selection. These
// types are the shared vocabulary between a request parser (such as
// binding.ParseRequest) and the Generator in the root sql_generator
// package, which compiles a Query into GORM scopes.
//
// Values of these types are typically constructed either by decoding a
// request (JSON body or, via the binding package, a URL query string) or
// by hand in tests and examples. The types themselves carry no validation
// logic — validation against a schema happens in Generator.Scopes.
package model

// SortDirection is the direction of a single Sort entry: Ascending or
// Descending.
type SortDirection string

// Condition is the boolean joiner used to combine multiple Filter or
// FilterGroup entries: And or Or.
type Condition string

// Operator identifies a comparison to apply between a field's column and
// a Filter's Value (or RangeValues). Operators are whitelisted per field
// in FieldMeta.Operators; a Filter using an operator not whitelisted for
// its field is rejected by Generator.Scopes.
type Operator string

// Comparison operators. Each maps to a specific SQL fragment when compiled
// by the Generator:
const (
	// IsEqual compiles to "column = ?".
	IsEqual Operator = "IS_EQUAL"
	// IsNotEqual compiles to "column <> ?".
	IsNotEqual Operator = "IS_NOT_EQUAL"
	// IsLessThan compiles to "column < ?".
	IsLessThan Operator = "IS_LESS_THAN"
	// IsMoreThan compiles to "column > ?".
	IsMoreThan Operator = "IS_MORE_THAN"
	// IsLessThanOrEqual compiles to "column <= ?".
	IsLessThanOrEqual Operator = "IS_LESS_THAN_OR_EQUAL"
	// IsMoreThanOrEqual compiles to "column >= ?".
	IsMoreThanOrEqual Operator = "IS_MORE_THAN_OR_EQUAL"
	// IsContain compiles to "column LIKE ?" with Value wrapped in "%...%".
	IsContain Operator = "IS_CONTAIN"
	// IsBeginWith compiles to "column LIKE ?" with Value suffixed by "%".
	IsBeginWith Operator = "IS_BEGIN_WITH"
	// IsEndWith compiles to "column LIKE ?" with Value prefixed by "%".
	IsEndWith Operator = "IS_END_WITH"
	// IsBetween compiles to "column BETWEEN ? AND ?" using RangeValues,
	// which must contain exactly two elements.
	IsBetween Operator = "IS_BETWEEN"
	// IsIn compiles to "column IN (?)" using RangeValues.
	IsIn Operator = "IS_IN"
	// IsNotIn compiles to "column NOT IN (?)" using RangeValues.
	IsNotIn Operator = "IS_NOT_IN"
	// IsNull compiles to "column IS NULL". Value and RangeValues are
	// ignored.
	IsNull Operator = "IS_NULL"
	// IsNotNull compiles to "column IS NOT NULL". Value and RangeValues
	// are ignored.
	IsNotNull Operator = "IS_NOT_NULL"
)

// Sort directions for a Sort entry.
const (
	// Ascending orders results low-to-high ("ORDER BY column ASC").
	Ascending SortDirection = "ASCENDING"
	// Descending orders results high-to-low ("ORDER BY column DESC").
	Descending SortDirection = "DESCENDING"
)

// Boolean joiners for combining sibling Filters within a FilterGroup.
const (
	// And requires every sibling condition to hold.
	And Condition = "AND"
	// Or requires at least one sibling condition to hold.
	Or Condition = "OR"
)
