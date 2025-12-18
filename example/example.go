package main

import (
	"fmt"
	"log"
	"sql-generator"
	"sql-generator/model"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Example User model
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

func main() {
	// Setup database connection
	dsn := "host=localhost user=postgres password=postgres dbname=testdb port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Auto migrate the schema
	db.AutoMigrate(&User{})

	// Configure the generator with schema metadata
	generator := &sql_generator.Generator{
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
					Searchable: true, // Included in global search
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
		CaseSensitiveSearch: false, // Use ILIKE for case-insensitive search
		MaxFiltersPerQuery:  50,    // Limit to 50 filters per query
		MaxSortsPerQuery:    10,    // Limit to 10 sorts per query
	}

	// Example 1: Simple filter query
	fmt.Println("=== Example 1: Simple Filter ===")
	simpleQuery := model.Query{
		SelectParameter: model.SelectParameter{
			Filters: []model.Filter{
				{
					FieldName: "status",
					Operator:  model.IsEqual,
					Value:     "active",
				},
			},
			PageDescriptor: model.Pagination{
				Page:     1,
				PageSize: 10,
			},
		},
	}
	executeQuery(db, generator, simpleQuery)

	// Example 2: Complex filter with AND/OR groups
	fmt.Println("\n=== Example 2: Complex Filter Groups ===")
	complexQuery := model.Query{
		SelectParameter: model.SelectParameter{
			FilterGroups: []model.FilterGroup{
				{
					Condition: model.Or,
					Filters: []model.Filter{
						{
							FieldName: "role",
							Operator:  model.IsEqual,
							Value:     "admin",
						},
						{
							FieldName: "role",
							Operator:  model.IsEqual,
							Value:     "moderator",
						},
					},
				},
				{
					Condition: model.And,
					Filters: []model.Filter{
						{
							FieldName: "status",
							Operator:  model.IsEqual,
							Value:     "active",
						},
						{
							FieldName:   "age",
							Operator:    model.IsBetween,
							RangeValues: []any{18, 65},
						},
					},
				},
			},
			PageDescriptor: model.Pagination{
				Page:     1,
				PageSize: 20,
			},
		},
	}
	executeQuery(db, generator, complexQuery)

	// Example 3: Global search
	fmt.Println("\n=== Example 3: Global Search ===")
	searchQuery := model.Query{
		Search: "john", // Searches all searchable fields (name, email)
		SelectParameter: model.SelectParameter{
			PageDescriptor: model.Pagination{
				Page:     1,
				PageSize: 10,
			},
		},
	}
	executeQuery(db, generator, searchQuery)

	// Example 4: Multiple sorts
	fmt.Println("\n=== Example 4: Multiple Sorts ===")
	sortQuery := model.Query{
		SelectParameter: model.SelectParameter{
			Sorts: []model.Sort{
				{
					FieldName:     "status",
					SortDirection: model.Ascending,
				},
				{
					FieldName:     "age",
					SortDirection: model.Descending,
				},
			},
			PageDescriptor: model.Pagination{
				Page:     1,
				PageSize: 10,
			},
		},
	}
	executeQuery(db, generator, sortQuery)

	// Example 5: Field projection (select specific columns)
	fmt.Println("\n=== Example 5: Field Projection ===")
	selectQuery := model.Query{
		SelectParameter: model.SelectParameter{
			Fields: []string{"id", "name", "email"}, // Only select these fields
			Filters: []model.Filter{
				{
					FieldName: "status",
					Operator:  model.IsEqual,
					Value:     "active",
				},
			},
			PageDescriptor: model.Pagination{
				Page:     1,
				PageSize: 10,
			},
		},
	}
	executeQuery(db, generator, selectQuery)

	// Example 6: Range filters (IN, BETWEEN)
	fmt.Println("\n=== Example 6: Range Filters ===")
	rangeQuery := model.Query{
		SelectParameter: model.SelectParameter{
			Filters: []model.Filter{
				{
					FieldName:   "status",
					Operator:    model.IsIn,
					RangeValues: []any{"active", "pending", "verified"},
				},
				{
					FieldName:   "age",
					Operator:    model.IsBetween,
					RangeValues: []any{25, 35},
				},
			},
			PageDescriptor: model.Pagination{
				Page:     1,
				PageSize: 10,
			},
		},
	}
	executeQuery(db, generator, rangeQuery)

	// Example 7: NULL checks
	fmt.Println("\n=== Example 7: NULL Checks ===")
	nullQuery := model.Query{
		SelectParameter: model.SelectParameter{
			Filters: []model.Filter{
				{
					FieldName: "deleted_at",
					Operator:  model.IsNull, // Only non-deleted records
				},
			},
			PageDescriptor: model.Pagination{
				Page:     1,
				PageSize: 10,
			},
		},
	}
	executeQuery(db, generator, nullQuery)

	// Example 8: Include soft-deleted records
	fmt.Println("\n=== Example 8: Include Soft-Deleted ===")
	softDeleteQuery := model.Query{
		IncludeDeleted: true, // Include soft-deleted records
		SelectParameter: model.SelectParameter{
			Filters: []model.Filter{
				{
					FieldName: "status",
					Operator:  model.IsEqual,
					Value:     "active",
				},
			},
			PageDescriptor: model.Pagination{
				Page:     1,
				PageSize: 10,
			},
		},
	}
	executeQuery(db, generator, softDeleteQuery)

	// Example 9: DISTINCT query
	fmt.Println("\n=== Example 9: Distinct Query ===")
	distinctQuery := model.Query{
		Distinct: true, // Apply DISTINCT
		SelectParameter: model.SelectParameter{
			Fields: []string{"status", "role"},
			PageDescriptor: model.Pagination{
				Page:     1,
				PageSize: 10,
			},
		},
	}
	executeQuery(db, generator, distinctQuery)

	// Example 10: String matching operators
	fmt.Println("\n=== Example 10: String Matching ===")
	stringQuery := model.Query{
		SelectParameter: model.SelectParameter{
			Filters: []model.Filter{
				{
					FieldName: "email",
					Operator:  model.IsEndWith,
					Value:     "@example.com",
				},
				{
					FieldName: "name",
					Operator:  model.IsContain,
					Value:     "smith",
				},
			},
			PageDescriptor: model.Pagination{
				Page:     1,
				PageSize: 10,
			},
		},
	}
	executeQuery(db, generator, stringQuery)
}

func executeQuery(db *gorm.DB, gen *sql_generator.Generator, query model.Query) {
	// Generate scopes from the query
	scopes, err := gen.Scopes(query)
	if err != nil {
		log.Printf("Error generating scopes: %v\n", err)
		return
	}

	// Apply scopes and execute query
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
