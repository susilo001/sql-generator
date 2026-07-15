package sql_generator

import (
	"reflect"
	"testing"

	"github.com/susilo001/sql-generator/model"
)

// testField returns a reflect.StructField for testing purposes.
func testField(name, gormTag string) reflect.StructField {
	type TestStruct struct {
		Field string `gorm:"column:test_column"`
	}
	f, _ := reflect.TypeOf(TestStruct{}).FieldByName("Field")
	f.Name = name
	if gormTag != "" {
		f.Tag = reflect.StructTag(`gorm:"` + gormTag + `"`)
	}
	return f
}

func TestParseFieldPart_Filter(t *testing.T) {
	f := testField("Name", "")
	
	tests := []struct {
		name      string
		part      tagPart
		wantOps   []model.Operator
		wantError bool
	}{
		{
			name:    "single operator",
			part:    tagPart{key: "filter", value: "eq", hasValue: true},
			wantOps: []model.Operator{model.IsEqual},
		},
		{
			name:    "multiple operators",
			part:    tagPart{key: "filter", value: "eq,ne,gt", hasValue: true},
			wantOps: []model.Operator{model.IsEqual, model.IsNotEqual, model.IsMoreThan},
		},
		{
			name:    "preset all",
			part:    tagPart{key: "filter", value: "all", hasValue: true},
			wantOps: []model.Operator{model.IsEqual, model.IsNotEqual, model.IsLessThan, model.IsMoreThan}, // partial check
		},
		{
			name:    "preset comparable",
			part:    tagPart{key: "filter", value: "comparable", hasValue: true},
			wantOps: []model.Operator{model.IsEqual, model.IsNotEqual, model.IsLessThan, model.IsMoreThan, model.IsBetween},
		},
		{
			name:    "preset text",
			part:    tagPart{key: "filter", value: "text", hasValue: true},
			wantOps: []model.Operator{model.IsEqual, model.IsContain, model.IsBeginWith, model.IsEndWith},
		},
		{
			name:      "missing value",
			part:      tagPart{key: "filter", hasValue: false},
			wantError: true,
		},
		{
			name:      "unknown operator",
			part:      tagPart{key: "filter", value: "invalid", hasValue: true},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseFieldPart(tt.part, f, "table", nil)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Operators == nil {
				t.Fatalf("expected Operators to be non-nil")
			}
			for _, op := range tt.wantOps {
				if !result.Operators[op] {
					t.Errorf("expected operator %v to be allowed", op)
				}
			}
		})
	}
}

func TestParseFieldPart_Search(t *testing.T) {
	f := testField("Name", "")
	
	t.Run("search", func(t *testing.T) {
		result, err := parseFieldPart(tagPart{key: "search"}, f, "table", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Searchable == nil || !*result.Searchable {
			t.Errorf("expected Searchable=true")
		}
	})

	t.Run("nosearch", func(t *testing.T) {
		result, err := parseFieldPart(tagPart{key: "nosearch"}, f, "table", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Searchable == nil || *result.Searchable {
			t.Errorf("expected Searchable=false")
		}
	})
}

func TestParseFieldPart_Column(t *testing.T) {
	f := testField("Name", "")
	
	tests := []struct {
		name      string
		part      tagPart
		want      string
		wantError bool
	}{
		{
			name: "valid column",
			part: tagPart{key: "column", value: "custom_col", hasValue: true},
			want: "custom_col",
		},
		{
			name:      "empty column",
			part:      tagPart{key: "column", value: "", hasValue: true},
			wantError: true,
		},
		{
			name:      "missing value",
			part:      tagPart{key: "column", hasValue: false},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseFieldPart(tt.part, f, "table", nil)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Column == nil || *result.Column != tt.want {
				t.Errorf("expected Column=%q, got %v", tt.want, result.Column)
			}
		})
	}
}

func TestParseFieldPart_Name(t *testing.T) {
	f := testField("Name", "")
	
	tests := []struct {
		name      string
		part      tagPart
		want      string
		wantError bool
	}{
		{
			name: "valid name",
			part: tagPart{key: "name", value: "alias", hasValue: true},
			want: "alias",
		},
		{
			name:      "empty name",
			part:      tagPart{key: "name", value: "", hasValue: true},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseFieldPart(tt.part, f, "table", nil)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Name == nil || *result.Name != tt.want {
				t.Errorf("expected Name=%q, got %v", tt.want, result.Name)
			}
		})
	}
}

func TestParseFieldPart_Joins(t *testing.T) {
	f := testField("Name", "")
	registry := map[string]JoinMeta{
		"details": {Table: "details", On: "details.id = main.detail_id", Type: "LEFT"},
		"other":   {Table: "other", On: "other.id = main.other_id", Type: "LEFT"},
	}

	tests := []struct {
		name      string
		part      tagPart
		wantCount int
		wantError bool
	}{
		{
			name:      "single join",
			part:      tagPart{key: "joins", value: "details", hasValue: true},
			wantCount: 1,
		},
		{
			name:      "multiple joins",
			part:      tagPart{key: "joins", value: "details,other", hasValue: true},
			wantCount: 2,
		},
		{
			name:      "unknown join",
			part:      tagPart{key: "joins", value: "unknown", hasValue: true},
			wantError: true,
		},
		{
			name:      "missing value",
			part:      tagPart{key: "joins", hasValue: false},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseFieldPart(tt.part, f, "table", registry)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.ExtraJoins) != tt.wantCount {
				t.Errorf("expected %d joins, got %d", tt.wantCount, len(result.ExtraJoins))
			}
		})
	}
}

func TestParseFieldPart_Exclusion(t *testing.T) {
	f := testField("Name", "")
	_, err := parseFieldPart(tagPart{key: "-"}, f, "table", nil)
	if err == nil || err.Error() != "field excluded by tag" {
		t.Errorf("expected exclusion error, got %v", err)
	}
}

func TestParseFieldPart_Unknown(t *testing.T) {
	f := testField("Name", "")
	_, err := parseFieldPart(tagPart{key: "invalid", raw: "invalid:value"}, f, "table", nil)
	if err == nil {
		t.Errorf("expected error for unknown tag part")
	}
}

func TestParseJoinPart_Join(t *testing.T) {
	tests := []struct {
		name      string
		part      tagPart
		want      string
		wantError bool
	}{
		{
			name: "left join",
			part: tagPart{key: "join", value: "left", hasValue: true},
			want: "LEFT",
		},
		{
			name: "inner join",
			part: tagPart{key: "join", value: "inner", hasValue: true},
			want: "INNER",
		},
		{
			name:      "invalid type",
			part:      tagPart{key: "join", value: "outer", hasValue: true},
			wantError: true,
		},
		{
			name:  "missing value defaults to left",
			part:  tagPart{key: "join", hasValue: false},
			want:  "LEFT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseJoinPart(tt.part)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.JoinType == nil || *result.JoinType != tt.want {
				t.Errorf("expected JoinType=%q, got %v", tt.want, result.JoinType)
			}
		})
	}
}

func TestParseJoinPart_Table(t *testing.T) {
	result, err := parseJoinPart(tagPart{key: "table", value: "users u", hasValue: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Table == nil || *result.Table != "users u" {
		t.Errorf("expected Table=%q, got %v", "users u", result.Table)
	}

	_, err = parseJoinPart(tagPart{key: "table", value: "", hasValue: true})
	if err == nil {
		t.Errorf("expected error for empty table")
	}
}

func TestParseJoinPart_On(t *testing.T) {
	result, err := parseJoinPart(tagPart{key: "on", value: "u.id = o.user_id", hasValue: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.On == nil || *result.On != "u.id = o.user_id" {
		t.Errorf("expected On=%q, got %v", "u.id = o.user_id", result.On)
	}

	_, err = parseJoinPart(tagPart{key: "on", value: "", hasValue: true})
	if err == nil {
		t.Errorf("expected error for empty on clause")
	}
}

func TestParseJoinPart_Leftover(t *testing.T) {
	result, err := parseJoinPart(tagPart{key: "filter", value: "eq", hasValue: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsLeftover {
		t.Errorf("expected IsLeftover=true for non-join tag part")
	}
}
