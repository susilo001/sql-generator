package sql_generator

import (
	"strings"
	"testing"
	"time"

	"github.com/susilo001/sql-generator/model"
)

// TaggedEmbedded mimics gorm.Model: an exported embedded struct whose
// promoted fields become schema entries.
type TaggedEmbedded struct {
	CreatedAt time.Time
	UpdatedAt time.Time `sqlgen:"-"`
}

type taggedProduct struct {
	TaggedEmbedded
	ID         int        `gorm:"column:id"`
	TypeID     int        `gorm:"column:type_id" sqlgen:"filter:eq,ne,in"`
	Name       string     `sqlgen:"filter:text;search"`
	DataStatus string     `gorm:"column:data_status" sqlgen:"filter:eq,ne,in,notin;nosearch"`
	DeletedAt  *time.Time `sqlgen:"filter:null,notnull"`
	Alias      string     `sqlgen:"column:external_name;filter:eq;nosearch"`
	Internal   string     `sqlgen:"-"`
	private    string
}

func TestFromModelBuildsSchemaFromStructTags(t *testing.T) {
	gen, err := FromModel(&taggedProduct{}, Options{
		DefaultFieldForSort:     "id",
		CaseSensitiveSearch:     true,
		MaxFiltersPerQuery:      10,
		MaxSortsPerQuery:        3,
		MaxFilterGroupDepth:     4,
		MaxFilterGroupsPerQuery: 5,
		MaxPageSize:             100,
		MaxRangeValues:          20,
		AllowedPreloads:         map[string]bool{"Type": true},
	})
	if err != nil {
		t.Fatalf("FromModel: %v", err)
	}

	if gen.DefaultFieldForSort != "id" || !gen.CaseSensitiveSearch {
		t.Fatalf("options not copied: %+v", gen)
	}
	if gen.MaxFiltersPerQuery != 10 || gen.MaxSortsPerQuery != 3 || gen.MaxFilterGroupDepth != 4 || gen.MaxFilterGroupsPerQuery != 5 || gen.MaxPageSize != 100 || gen.MaxRangeValues != 20 {
		t.Fatalf("limits not copied: %+v", gen)
	}
	if !gen.Schema.AllowedPreloads["Type"] {
		t.Fatalf("allowed preloads not copied")
	}

	fields := gen.Schema.Fields
	for _, name := range []string{"created_at", "id", "type_id", "name", "data_status", "deleted_at", "external_name"} {
		if _, ok := fields[name]; !ok {
			t.Fatalf("expected field %q in schema", name)
		}
	}
	for _, name := range []string{"updated_at", "internal", "private"} {
		if _, ok := fields[name]; ok {
			t.Fatalf("did not expect field %q in schema", name)
		}
	}

	if !fields["id"].Operators[model.IsContain] || !fields["id"].Operators[model.IsNotNull] {
		t.Fatalf("untagged id should allow all operators")
	}
	if !fields["name"].Searchable {
		t.Fatalf("name should be searchable")
	}
	if fields["data_status"].Searchable {
		t.Fatalf("data_status should not be searchable")
	}
	if !fields["type_id"].Operators[model.IsEqual] || !fields["type_id"].Operators[model.IsNotEqual] || !fields["type_id"].Operators[model.IsIn] {
		t.Fatalf("type_id operator aliases not parsed: %#v", fields["type_id"].Operators)
	}
	if fields["type_id"].Operators[model.IsContain] {
		t.Fatalf("type_id should not allow contains")
	}
	if !fields["deleted_at"].Operators[model.IsNull] || !fields["deleted_at"].Operators[model.IsNotNull] {
		t.Fatalf("deleted_at null operators not parsed")
	}
}

func TestFromModelEndToEndScopes(t *testing.T) {
	gen, err := FromModel(&taggedProduct{}, Options{
		DefaultFieldForSort: "id",
		CaseSensitiveSearch: true,
	})
	if err != nil {
		t.Fatalf("FromModel: %v", err)
	}

	sql, vars := buildSQL(t, gen, model.Query{
		Search: "capital",
		SelectParameter: model.SelectParameter{
			Filters: []model.Filter{
				{FieldName: "type_id", Operator: model.IsEqual, Value: 1},
				{FieldName: "data_status", Operator: model.IsEqual, Value: "Active"},
			},
			Sorts:          []model.Sort{{FieldName: "type_id", SortDirection: model.Ascending}},
			PageDescriptor: model.Pagination{Page: 2, PageSize: 10},
		},
	})

	for _, want := range []string{"WHERE", "type_id = ?", "data_status = ?", "name LIKE ?", "ORDER BY type_id ASC", "LIMIT ? OFFSET ?"} {
		if !strings.Contains(sql, want) {
			t.Fatalf("SQL missing %q: %s", want, sql)
		}
	}
	if len(vars) != 5 {
		t.Fatalf("vars len = %d, want 5 (2 filters, 1 search, limit, offset): %#v", len(vars), vars)
	}
	if vars[0] != 1 || vars[1] != "Active" || vars[2] != "%capital%" {
		t.Fatalf("unexpected vars: %#v", vars)
	}
}

func TestFromModelRejectsMalformedTags(t *testing.T) {
	tests := []struct {
		name  string
		model any
		want  string
	}{
		{
			name: "unknown operator",
			model: struct {
				Name string `sqlgen:"filter:wat"`
			}{},
			want: "unknown filter operator",
		},
		{
			name: "unknown part",
			model: struct {
				Name string `sqlgen:"sortable"`
			}{},
			want: "unknown tag part",
		},
		{
			name: "empty column",
			model: struct {
				Name string `sqlgen:"column:"`
			}{},
			want: "column tag needs a name",
		},
		{
			name: "duplicate column",
			model: struct {
				A string `sqlgen:"column:name"`
				B string `gorm:"column:name"`
			}{},
			want: "duplicate column",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FromModel(tt.model)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("FromModel error = %v, want contains %q", err, tt.want)
			}
		})
	}
}

func TestFromModelRequiresStruct(t *testing.T) {
	_, err := FromModel(42)
	if err == nil || !strings.Contains(err.Error(), "expected a struct") {
		t.Fatalf("FromModel error = %v, want struct error", err)
	}
}
