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
			name: "duplicate query-facing name",
			model: struct {
				A string `sqlgen:"name:name"`
				B string `sqlgen:"name:name"`
			}{},
			want: "duplicate field name",
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

// mutualFundDetail / bondDetail model the joined-table shape from a real
// repository schema: nested structs with a join tag on the parent field.
type mutualFundDetail struct {
	FundCategory      string `sqlgen:"filter:eq,in,contains,null,notnull;search"`
	InvestmentManager string `sqlgen:"filter:eq,contains,startswith,null,notnull;search"`
	IsSyariah         bool   `sqlgen:"filter:eq;name:is_syariah;column:COALESCE(mutual_fund_details.is_syariah, bond_details.is_syariah);joins:bond_details"`
}

type bondDetail struct {
	BondType string `sqlgen:"filter:eq,in,contains,null,notnull;search"`
	Issuer   string `sqlgen:"filter:eq,contains,startswith,null,notnull;search"`
	IsRetail bool   `sqlgen:"filter:eq"`
}

type productType struct {
	ID int
}

type investmentProduct struct {
	ID         int              `sqlgen:"filter:eq,in"`
	TenantCode string           `sqlgen:"filter:eq,in,contains;search"`
	Name       string           `sqlgen:"filter:eq,contains,startswith;search"`
	RiskLevel  int              `sqlgen:"filter:comparable"`
	Type       *productType     // association without join tag: skipped
	Tags       []string         // slice association: skipped
	Fund       mutualFundDetail `sqlgen:"join:left;table:mutual_fund_details;on:mutual_fund_details.product_id = investment_products.id"`
	Bond       *bondDetail      `sqlgen:"join:left;table:bond_details;on:bond_details.product_id = investment_products.id"`
}

func TestFromModelJoinsAndTableQualification(t *testing.T) {
	gen, err := FromModel(&investmentProduct{}, Options{
		Table:               "investment_products",
		DefaultFieldForSort: "investment_products.created_at",
		CaseSensitiveSearch: true,
	})
	if err != nil {
		t.Fatalf("FromModel: %v", err)
	}
	fields := gen.Schema.Fields

	// Root columns qualified with the base table; field names stay bare.
	if got := fields["id"].Column; got != "investment_products.id" {
		t.Fatalf("id column = %q", got)
	}
	if got := fields["tenant_code"].Column; got != "investment_products.tenant_code" {
		t.Fatalf("tenant_code column = %q", got)
	}

	// Associations without a join tag are skipped.
	if _, ok := fields["type"]; ok {
		t.Fatalf("association Type should be skipped")
	}
	if _, ok := fields["tags"]; ok {
		t.Fatalf("slice Tags should be skipped")
	}

	// Joined leaf fields: qualified with the join table + JoinMeta attached.
	fc, ok := fields["fund_category"]
	if !ok {
		t.Fatalf("fund_category missing: %v", keys(fields))
	}
	if fc.Column != "mutual_fund_details.fund_category" {
		t.Fatalf("fund_category column = %q", fc.Column)
	}
	if fc.Join == nil || fc.Join.Table != "mutual_fund_details" || fc.Join.Type != "LEFT" ||
		fc.Join.On != "mutual_fund_details.product_id = investment_products.id" {
		t.Fatalf("fund_category join = %+v", fc.Join)
	}

	// Pointer-to-struct joins work too.
	if bt := fields["bond_type"]; bt.Join == nil || bt.Join.Table != "bond_details" {
		t.Fatalf("bond_type join = %+v", fields["bond_type"].Join)
	}

	// name: + expression column: used verbatim, never re-qualified.
	is, ok := fields["is_syariah"]
	if !ok {
		t.Fatalf("is_syariah missing: %v", keys(fields))
	}
	if is.Column != "COALESCE(mutual_fund_details.is_syariah, bond_details.is_syariah)" {
		t.Fatalf("is_syariah column = %q", is.Column)
	}
	if !is.Operators[model.IsEqual] || is.Operators[model.IsContain] {
		t.Fatalf("is_syariah operators = %#v", is.Operators)
	}

	// End-to-end: filtering on a joined field auto-applies the JOIN.
	sql, _ := buildSQL(t, gen, model.Query{
		SelectParameter: model.SelectParameter{
			Filters: []model.Filter{
				{FieldName: "fund_category", Operator: model.IsEqual, Value: "equity"},
				{FieldName: "id", Operator: model.IsEqual, Value: 7},
			},
		},
	})
	for _, want := range []string{
		"LEFT JOIN mutual_fund_details ON mutual_fund_details.product_id = investment_products.id",
		"mutual_fund_details.fund_category = ?",
		"investment_products.id = ?",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("SQL missing %q: %s", want, sql)
		}
	}

	// Expression columns can require joins beyond their enclosing struct.
	sql, _ = buildSQL(t, gen, model.Query{
		SelectParameter: model.SelectParameter{
			Filters: []model.Filter{{FieldName: "is_syariah", Operator: model.IsEqual, Value: true}},
		},
	})
	for _, want := range []string{
		"LEFT JOIN mutual_fund_details ON mutual_fund_details.product_id = investment_products.id",
		"LEFT JOIN bond_details ON bond_details.product_id = investment_products.id",
		"COALESCE(mutual_fund_details.is_syariah, bond_details.is_syariah) = ?",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("SQL missing %q: %s", want, sql)
		}
	}
}

func TestFromModelJoinAliasQualification(t *testing.T) {
	type detail struct {
		Code string `sqlgen:"filter:eq"`
	}
	type product struct {
		Detail detail `sqlgen:"join:left;table:product_details pd;on:pd.product_id = products.id"`
	}

	gen, err := FromModel(&product{}, Options{Table: "products"})
	if err != nil {
		t.Fatalf("FromModel: %v", err)
	}
	if got := gen.Schema.Fields["code"].Column; got != "pd.code" {
		t.Fatalf("joined alias column = %q, want pd.code", got)
	}
	sql, _ := buildSQL(t, gen, model.Query{
		SelectParameter: model.SelectParameter{
			Filters: []model.Filter{{FieldName: "code", Operator: model.IsEqual, Value: "x"}},
		},
	})
	for _, want := range []string{
		"LEFT JOIN product_details pd ON pd.product_id = products.id",
		"pd.code = ?",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("SQL missing %q: %s", want, sql)
		}
	}
}

func TestFromModelJoinTagErrors(t *testing.T) {
	tests := []struct {
		name  string
		model any
		want  string
	}{
		{name: "join missing on", model: struct {
			Fund mutualFundDetail `sqlgen:"join:left;table:mutual_fund_details"`
		}{}, want: "join tag requires table: and on:"},
		{name: "join on non-struct", model: struct {
			Name string `sqlgen:"join:left;table:t;on:t.id = x.id"`
		}{}, want: "join tag requires a struct field"},
		{name: "unknown join type", model: struct {
			Fund mutualFundDetail `sqlgen:"join:cross;table:t;on:t.id = x.id"`
		}{}, want: "unknown join type"},
		{name: "join mixed with filter reports unknown tag part", model: struct {
			Fund mutualFundDetail `sqlgen:"join:left;table:t;on:t.id = x.id;filter:eq"`
		}{}, want: "unknown tag part"},
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

func keys(m map[string]FieldMeta) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestFromModelRequiresStruct(t *testing.T) {
	_, err := FromModel(42)
	if err == nil || !strings.Contains(err.Error(), "expected a struct") {
		t.Fatalf("FromModel error = %v, want struct error", err)
	}
}
