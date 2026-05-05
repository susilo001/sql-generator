package model

type Filter struct {
	FieldName   string    `json:"field_name"`
	Operator    Operator  `json:"operator"`
	Condition   Condition `json:"condition"`
	Value       any       `json:"value"`
	Value2      any       `json:"value2"`
	RangeValues []any     `json:"range_values"`
}

type FilterGroup struct {
	Condition    Condition     `json:"condition"`
	Filters      []Filter      `json:"filters"`
	FilterGroups []FilterGroup `json:"filter_groups"`
}
