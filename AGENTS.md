# sql-generator — Project Summary & Directions

## Project Overview

**sql-generator** is a Go library (v2.0.0) that translates frontend request parameters into safe, composable GORM query scopes. It sits between an HTTP handler and a GORM database call, handling filtering, sorting, pagination, joins, and projection so API handlers don't have to.

**Design philosophy:**
- **Schema-driven** — a `ModelMeta` struct declares which fields exist, which operators are allowed per field, and how joins work. Anything not declared is rejected.
- **Scope-builder pattern** — `generator.Scopes(q model.Query)` returns `[]func(*gorm.DB) *gorm.DB`, composable GORM scopes applied via `db.Scopes(scopes...).Find(&results)`.
- **Security-first** — field whitelisting, per-field operator whitelisting, configurable filter/sort count limits, and GORM parameterized queries throughout.

---

## Current Capabilities (v2.0.0)

### Filtering
| Feature | Description |
|---|---|
| 14 operators | `IS_EQUAL`, `IS_NOT_EQUAL`, `IS_LESS_THAN`, `IS_MORE_THAN`, `IS_LESS_THAN_OR_EQUAL`, `IS_MORE_THAN_OR_EQUAL`, `IS_CONTAIN`, `IS_BEGIN_WITH`, `IS_END_WITH`, `IS_BETWEEN`, `IS_IN`, `IS_NOT_IN`, `IS_NULL`, `IS_NOT_NULL` |
| Simple filters | Flat list of filters, all ANDed together |
| FilterGroups | Nested AND/OR logic with unlimited nesting depth |
| Global search | Single search term applied across all `Searchable` fields |
| Case sensitivity | Configurable: ILIKE (default) or LIKE |

### Data Retrieval
| Feature | Description |
|---|---|
| Multi-field sorting | Multiple `Sort` entries with `ASCENDING` / `DESCENDING` per field |
| Offset pagination | Page + PageSize with safe defaults (page 1, size 20) |
| Field projection | SELECT specific columns via `Fields []string` |
| Eager loading | GORM `Preload` relationships via `Preloads []string` |
| Automatic joins | LEFT / INNER join auto-applied when a joined field is referenced |
| DISTINCT | Remove duplicate rows |
| Soft delete | `IncludeDeleted` flag calls GORM `Unscoped()` |

### Security
- Field and operator whitelisting via `ModelMeta.Fields` and `FieldMeta.Operators`
- `MaxFiltersPerQuery` and `MaxSortsPerQuery` with recursive filter counting
- All values passed as GORM parameterized arguments (no string concatenation)

---

## Architecture

```
model/
  enums.go        — Operator, SortDirection, Condition constants
  filter.go       — Filter and FilterGroup types
  query.go        — Query and SelectParameter types
  sort.go         — Sort type
  pagination.go   — Pagination type

generator.go      — Generator struct + Scopes() entry point + all scope builders
generator_test.go — Test suite (~10 test groups covering all features)
example/
  example.go      — 10 runnable usage examples
```

**Entry point:**
```go
scopes, err := generator.Scopes(q model.Query)
// → []func(*gorm.DB) *gorm.DB
db.Scopes(scopes...).Find(&results)
```

**Configuration:**
```go
gen := sql_generator.Generator{
    Schema:              &modelMeta,        // required: field whitelist + join metadata
    DefaultFieldForSort: "created_at",      // fallback sort column
    CaseSensitiveSearch: false,             // ILIKE vs LIKE
    MaxFiltersPerQuery:  20,                // 0 = unlimited
    MaxSortsPerQuery:    5,
}
```

---

## Possible Directions

### A — Driver-Agnostic Support
**What:** Remove the hard dependency on `gorm.io/driver/postgres`. Let callers supply any GORM-supported driver (MySQL, SQLite, SQL Server).

**Why it matters:** Currently `go.mod` pulls in the Postgres driver unconditionally. Teams using MySQL or SQLite must carry an unused dependency and work around dialect differences (e.g., `ILIKE` is Postgres-only).

**Trade-offs:** Dialect differences require conditional logic or a dialect interface — `ILIKE` becomes `LIKE LOWER(?)` on MySQL, date functions vary. Adds a non-trivial abstraction layer.

---

### B — HTTP Middleware / Framework Binding Layer
**What:** Add a thin binding helper (or per-framework middleware) that parses a JSON request body directly into `model.Query`, ready to pass to `Scopes()`.

**Why it matters:** Every API handler currently repeats the same JSON decode + `Scopes()` call pattern. `example/example.go` already shows it. Extracting it reduces boilerplate and standardizes error responses.

**Trade-offs:** Supporting Gin, Echo, and Fiber means either multiple optional sub-packages or a framework-agnostic `io.Reader` approach. Adds framework dependencies or forces callers to choose.

---

### C — OpenAPI / JSON Schema Generation
**What:** Add a function that takes a `ModelMeta` and emits an OpenAPI schema or JSON Schema document describing which fields, operators, and sort directions are valid for a given model.

**Why it matters:** Frontends currently have to discover valid query parameters by trial and error or read Go source. A generated schema creates a machine-readable API contract.

**Trade-offs:** Schema generation is complex and the output format (OpenAPI 3.x vs JSON Schema) is a significant choice. Useful for API-first teams, overkill for internal tooling.

---

### D — Aggregation & Group-By Support
**What:** Extend `Query` and `Generator` to support `COUNT`, `SUM`, `AVG`, `MIN`, `MAX`, and `GROUP BY` — analytics-style queries alongside the existing list queries.

**Why it matters:** Once row-level listing is covered, the next common need is aggregate dashboards. Currently callers have to drop out of `sql-generator` entirely for any aggregate query.

**Trade-offs:** Significantly expands the API surface. Aggregates interact with filters and joins in complex ways, and maintaining schema-safety for arbitrary aggregate expressions is hard.

---

### E — Cursor-Based (Keyset) Pagination
**What:** Add an alternative to offset pagination using an opaque cursor (encoded last-seen key values) for stable, performant pagination over large result sets.

**Why it matters:** Offset pagination (`LIMIT x OFFSET y`) degrades on large tables — the database must scan and discard rows. Keyset pagination is O(log n) and stable under inserts.

**Trade-offs:** Frontends can no longer jump to arbitrary pages — they navigate forward/backward only. Requires a stable sort column (typically the primary key). Breaks the current simple `page`/`pageSize` UX.

---

### F — Scope Generation Caching
**What:** Cache the result of scope generation keyed by a hash of `ModelMeta` + `Query`, so repeated identical queries skip validation and scope construction.

**Why it matters:** Scope generation is deterministic and pure. Under high request volume, caching could reduce CPU work per request.

**Trade-offs:** Scope functions close over query values, making them non-trivially cacheable (you'd cache the configuration, not the closures). Marginal gain unless profiling shows scope generation as a bottleneck. Adds complexity for uncertain benefit.

---

### G — CLI / Code Generator for ModelMeta
**What:** A standalone CLI tool (`sql-gen`) that reads a GORM model struct (via AST parsing or reflection at runtime) and generates the `ModelMeta` + `Generator` initialization code.

**Why it matters:** Writing `ModelMeta` by hand is the most friction-heavy part of adopting the library. Auto-generating it from a GORM model keeps the schema in sync with the struct automatically.

**Trade-offs:** Requires Go AST parsing (compile-time) or `reflect` + manual type inspection (runtime). Codegen tools add a build-step dependency and their output needs to be committed or re-generated on model changes. Significant investment for a DX improvement.

---

## Decision Criteria

When choosing a direction, consider:

| Question | Guidance |
|---|---|
| Who is blocked right now? | **B** (middleware) removes the most repeated boilerplate for existing users |
| What broadens adoption most? | **A** (driver agnostic) removes the biggest adoption barrier for non-Postgres teams |
| What do analytics users need? | **D** (aggregation) is the natural feature gap once listing is solid |
| What improves scale? | **E** (cursor pagination) matters when result sets exceed tens of thousands of rows |
| What improves DX the most? | **G** (CLI codegen) reduces setup friction for new adopters |
