package model

type Sort struct {
	FieldName     string        `json:"field_name"`
	SortDirection SortDirection `json:"sort_direction"`
}
