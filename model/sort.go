package model

// Sort is a single "order by" entry: order results by FieldName in
// SortDirection. FieldName is a query-facing field name that must be
// declared in the target ModelMeta.Fields; if it is not, Generator's
// sortScope silently skips this entry rather than erroring.
type Sort struct {
	FieldName     string        `json:"field_name"`
	SortDirection SortDirection `json:"sort_direction"`
}
