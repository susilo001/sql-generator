// Command example demonstrates sql-generator end to end: declaring a
// ModelMeta schema, building queries programmatically, parsing a query
// straight from an HTTP URL via the binding package, and applying the
// resulting GORM scopes against a database.
//
// Run with a reachable Postgres instance:
//
//	go run ./example
package main

import (
	"fmt"
	"log"
	"net/http"

	sql_generator "github.com/susilo001/sql-generator"
	"github.com/susilo001/sql-generator/binding"
	"github.com/susilo001/sql-generator/model"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// User is the example GORM model these queries run against.
type User struct {
	ID        uint `gorm:"primaryKey"`
	Name      string
	Email     string
	Status    string
	Age       int
	Role      string
	DeletedAt *gorm.DeletedAt `gorm:"index"`
}

func (u User) TableName() string {
	return "user"
}

// newGenerator builds the Generator + ModelMeta schema shared by every
// example below. In a real project this typically lives once per model,
// next to the repository that uses it.
func newGenerator() *sql_generator.Generator {
	return &sql_generator.Generator{
		Schema: &sql_generator.ModelMeta{
			Fields: map[string]sql_generator.FieldMeta{
				"id": {
					Column:     "id",
					Searchable: false,
					Operators: map[model.Operator]bool{
						model.IsEqual:    true,
						model.IsNotEqual: true,
						model.IsIn:       true,
					},
				},
				"name": {
					Column:     "name",
					Searchable: true, // included in global Search
					Operators: map[model.Operator]bool{
						model.IsEqual:     true,
						model.IsNotEqual:  true,
						model.IsContain:   true,
						model.IsBeginWith: true,
						model.IsEndWith:   true,
					},
				},
				"email": {
					Column:     "email",
					Searchable: true,
					Operators: map[model.Operator]bool{
						model.IsEqual:     true,
						model.IsContain:   true,
						model.IsBeginWith: true,
						model.IsEndWith:   true,
					},
				},
				"status": {
					Column:     "status",
					Searchable: false,
					Operators: map[model.Operator]bool{
						model.IsEqual:    true,
						model.IsNotEqual: true,
						model.IsIn:       true,
						model.IsNotIn:    true,
					},
				},
				"age": {
					Column:     "age",
					Searchable: false,
					Operators: map[model.Operator]bool{
						model.IsEqual:           true,
						model.IsMoreThan:        true,
						model.IsLessThan:        true,
						model.IsMoreThanOrEqual: true,
						model.IsLessThanOrEqual: true,
						model.IsBetween:         true,
					},
				},
				"role": {
					Column:     "role",
					Searchable: false,
					Operators: map[model.Operator]bool{
						model.IsEqual:    true,
						model.IsNotEqual: true,
						model.IsIn:       true,
					},
				},
				"deleted_at": {
					Column:     "deleted_at",
					Searchable: false,
					Operators: map[model.Operator]bool{
						model.IsNull:    true,
						model.IsNotNull: true,
					},
				},
			},
		},
		DefaultFieldForSort: "id",
		CaseSensitiveSearch: false, // ILIKE (Postgres) for case-insensitive search
		MaxFiltersPerQuery:  50,
		MaxSortsPerQuery:    10,
	}
}

func main() {
	dsn := "host=localhost user=myuser password=mypassword dbname=mydb port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	if err := db.AutoMigrate(&User{}); err != nil {
		log.Fatal("Failed to migrate schema:", err)
	}

	gen := newGenerator()

	fmt.Println("=== Example 1: Simple Filter ===")
	executeQuery(db, gen, model.Query{
		SelectParameter: model.SelectParameter{
			Filters: []model.Filter{
				{FieldName: "status", Operator: model.IsEqual, Value: "active"},
			},
			PageDescriptor: model.Pagination{Page: 1, PageSize: 10},
		},
	})

	fmt.Println("\n=== Example 2: Nested FilterGroups (AND/OR) ===")
	// (role = admin OR role = moderator) OR (status = active AND age BETWEEN 18 AND 65)
	executeQuery(db, gen, model.Query{
		SelectParameter: model.SelectParameter{
			FilterGroups: []model.FilterGroup{
				{
					Condition: model.Or,
					Filters: []model.Filter{
						{FieldName: "role", Operator: model.IsEqual, Value: "admin"},
						{FieldName: "role", Operator: model.IsEqual, Value: "moderator"},
					},
					FilterGroups: []model.FilterGroup{
						{
							Condition: model.And,
							Filters: []model.Filter{
								{FieldName: "status", Operator: model.IsEqual, Value: "active"},
								{FieldName: "age", Operator: model.IsBetween, RangeValues: []any{18, 65}},
							},
						},
					},
				},
			},
			PageDescriptor: model.Pagination{Page: 1, PageSize: 20},
		},
	})

	fmt.Println("\n=== Example 3: Global Search ===")
	executeQuery(db, gen, model.Query{
		Search: "john", // matches any Searchable field: name, email
		SelectParameter: model.SelectParameter{
			PageDescriptor: model.Pagination{Page: 1, PageSize: 10},
		},
	})

	fmt.Println("\n=== Example 4: Multiple Sorts ===")
	executeQuery(db, gen, model.Query{
		SelectParameter: model.SelectParameter{
			Sorts: []model.Sort{
				{FieldName: "status", SortDirection: model.Ascending},
				{FieldName: "age", SortDirection: model.Descending},
			},
			PageDescriptor: model.Pagination{Page: 1, PageSize: 10},
		},
	})

	fmt.Println("\n=== Example 5: Field Projection ===")
	executeQuery(db, gen, model.Query{
		SelectParameter: model.SelectParameter{
			Fields: []string{"id", "name", "email"},
			Filters: []model.Filter{
				{FieldName: "status", Operator: model.IsEqual, Value: "active"},
			},
			PageDescriptor: model.Pagination{Page: 1, PageSize: 10},
		},
	})

	fmt.Println("\n=== Example 6: Range Filters (IN, BETWEEN) ===")
	executeQuery(db, gen, model.Query{
		SelectParameter: model.SelectParameter{
			Filters: []model.Filter{
				{FieldName: "status", Operator: model.IsIn, RangeValues: []any{"active", "pending", "verified"}},
				{FieldName: "age", Operator: model.IsBetween, RangeValues: []any{25, 35}},
			},
			PageDescriptor: model.Pagination{Page: 1, PageSize: 10},
		},
	})

	fmt.Println("\n=== Example 7: NULL Checks ===")
	executeQuery(db, gen, model.Query{
		SelectParameter: model.SelectParameter{
			Filters: []model.Filter{
				{FieldName: "deleted_at", Operator: model.IsNull},
			},
			PageDescriptor: model.Pagination{Page: 1, PageSize: 10},
		},
	})

	fmt.Println("\n=== Example 8: String Matching Operators ===")
	executeQuery(db, gen, model.Query{
		SelectParameter: model.SelectParameter{
			Filters: []model.Filter{
				{FieldName: "email", Operator: model.IsEndWith, Value: "@example.com"},
				{FieldName: "name", Operator: model.IsContain, Value: "smith"},
			},
			PageDescriptor: model.Pagination{Page: 1, PageSize: 10},
		},
	})

	fmt.Println("\n=== Example 9: Parsing a Query Straight From a URL ===")
	// Mirrors a real frontend request, e.g.:
	//   GET /v2/users?search=capital&sort=age:asc&pageSize=10&page=2&filter=status:active:equals|role:admin:notequals
	req, _ := http.NewRequest(http.MethodGet,
		"/v2/users?search=capital&sort=age:asc&pageSize=10&page=2&filter=status:active:equals|role:admin:notequals",
		nil)

	urlQuery, err := binding.ParseRequest(req, nil)
	if err != nil {
		log.Printf("Error parsing URL query: %v\n", err)
	} else {
		executeQuery(db, gen, urlQuery)
	}
}

// executeQuery compiles q into GORM scopes via gen.Scopes, applies them, and
// prints the results. Any schema validation error (unknown field, disallowed
// operator, malformed range, too many filters/sorts) is caught here rather
// than surfacing as a raw SQL error from the database.
func executeQuery(db *gorm.DB, gen *sql_generator.Generator, query model.Query) {
	scopes, err := gen.Scopes(query)
	if err != nil {
		log.Printf("Error generating scopes: %v\n", err)
		return
	}

	var users []User
	result := db.Scopes(scopes...).Find(&users)
	if result.Error != nil {
		log.Printf("Error executing query: %v\n", result.Error)
		return
	}

	fmt.Printf("Found %d users\n", len(users))
	for _, user := range users {
		fmt.Printf("  - %s (%s) - %s\n", user.Name, user.Email, user.Status)
	}
}
