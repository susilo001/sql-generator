//go:build ignore

// This file demonstrates how to wire sql-generator with common HTTP frameworks.
// It is excluded from the normal build (//go:build ignore) because it imports
// Gin and Fiber, which are not dependencies of this library.
//
// Run a specific example with: go run http_example.go
package main

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	sqlgenerator "github.com/susilo001/sql-generator"
	"github.com/susilo001/sql-generator/binding"
	"github.com/susilo001/sql-generator/model"
)

// --- Shared setup ----------------------------------------------------------

type Product struct {
	ID       uint `gorm:"primaryKey"`
	Name     string
	Category string
	Price    float64
	Stock    int
}

var productGenerator = sqlgenerator.Generator{
	Schema: &sqlgenerator.ModelMeta{
		Fields: map[string]sqlgenerator.FieldMeta{
			"name": {
				Column:     "name",
				Searchable: true,
				Operators: map[model.Operator]bool{
					model.IsEqual:   true,
					model.IsContain: true,
				},
			},
			"category": {
				Column: "category",
				Operators: map[model.Operator]bool{
					model.IsEqual: true,
					model.IsIn:    true,
				},
			},
			"price": {
				Column: "price",
				Operators: map[model.Operator]bool{
					model.IsLessThan:        true,
					model.IsMoreThan:        true,
					model.IsLessThanOrEqual: true,
					model.IsMoreThanOrEqual: true,
					model.IsBetween:         true,
				},
			},
			"stock": {
				Column: "stock",
				Operators: map[model.Operator]bool{
					model.IsMoreThan:        true,
					model.IsMoreThanOrEqual: true,
					model.IsEqual:           true,
				},
			},
		},
	},
	DefaultFieldForSort: "id",
	MaxFiltersPerQuery:  20,
	MaxSortsPerQuery:    5,
}

// --- Gin handler -----------------------------------------------------------

// GinListProducts is a Gin handler that accepts a JSON query body and returns
// a filtered, sorted, paginated list of products.
//
// POST /products/search
// Body: { "search": "...", "select_parameter": { "filters": [...], ... } }
func GinListProducts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		q, err := binding.ParseBody(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		scopes, err := productGenerator.Scopes(q)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var products []Product
		if err := db.Scopes(scopes...).Find(&products).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, products)
	}
}

// SetupGin wires the Gin router.
func SetupGin(db *gorm.DB) *gin.Engine {
	r := gin.Default()
	r.POST("/products/search", GinListProducts(db))
	return r
}

// --- Fiber handler ---------------------------------------------------------

// FiberListProducts is a Fiber handler that accepts a JSON query body and returns
// a filtered, sorted, paginated list of products.
//
// POST /products/search
// Body: { "search": "...", "select_parameter": { "filters": [...], ... } }
func FiberListProducts(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		q, err := binding.ParseBytes(c.Body())
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		scopes, err := productGenerator.Scopes(q)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		var products []Product
		if err := db.Scopes(scopes...).Find(&products).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "database error"})
		}

		return c.JSON(products)
	}
}

// SetupFiber wires the Fiber app.
func SetupFiber(db *gorm.DB) *fiber.App {
	app := fiber.New()
	app.Post("/products/search", FiberListProducts(db))
	return app
}

// --- net/http handler ------------------------------------------------------

// StdListProducts is a plain net/http handler — no framework needed.
//
// POST /products/search
func StdListProducts(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := binding.ParseBody(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		scopes, err := productGenerator.Scopes(q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var products []Product
		if err := db.Scopes(scopes...).Find(&products).Error; err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(products)
	}
}
