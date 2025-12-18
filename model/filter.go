package model

type Filter struct {
	FieldName   string
	Operator    Operator
	Condition   Condition
	Value       any
	Value2      any
	RangeValues []any
}

type FilterGroup struct {
	Condition    Condition     // AND / OR
	Filters      []Filter      // Individual filters
	FilterGroups []FilterGroup // Nested groups for complex logic
}
