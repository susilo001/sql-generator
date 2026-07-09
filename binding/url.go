// Package binding parses HTTP URL query strings into a model.Query.
//
// Grammar:
//
//	?search=<term>
//	&page=<int>
//	&pageSize=<int>
//	&sort=<field>:<asc|desc>[,<field>:<asc|desc>...]
//	&filter=<field>:<value>:<op>[|<field>:<value>:<op>...]
//	&fields=<field>[,<field>...]
//	&preloads=<name>[,<name>...]
//
// Operator aliases are case-insensitive. Values for isin/isnotin operators
// are split on comma into RangeValues.
package binding

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/susilo001/sql-generator/model"
)

// DefaultOpAliases is the default URL-alias → model.Operator map.
// Keys must be lowercase; lookup is case-insensitive.
var DefaultOpAliases = map[string]model.Operator{
	"equals":             model.IsEqual,
	"notequals":          model.IsNotEqual,
	"greaterthan":        model.IsMoreThan,
	"greaterthanorequal": model.IsMoreThanOrEqual,
	"lessthan":           model.IsLessThan,
	"lessthanorequal":    model.IsLessThanOrEqual,
	"contains":           model.IsContain,
	"startswith":         model.IsBeginWith,
	"endswith":           model.IsEndWith,
	"isin":               model.IsIn,
	"isnotin":            model.IsNotIn,
}

// URLParseOptions customizes URL query parsing.
//
// OpAliases fully REPLACES the default alias map when non-nil.
// Pass nil to use DefaultOpAliases.
type URLParseOptions struct {
	// Separators
	FilterSep    string // between filter clauses. Default "|"
	FilterTriple string // inside a filter clause. Default ":"
	SortSep      string // between sort clauses. Default ","
	RangeSep     string // inside isin/isnotin values. Default ","
	ListSep      string // for fields / preloads. Default ","

	// Query keys
	SearchKey   string // default "search"
	FilterKey   string // default "filter"
	SortKey     string // default "sort"
	PageKey     string // default "page"
	PageSizeKey string // default "pageSize"
	FieldsKey   string // default "fields"
	PreloadsKey string // default "preloads"

	// OpAliases replaces DefaultOpAliases when non-nil. Keys are matched
	// case-insensitively; supply lowercase keys.
	OpAliases map[string]model.Operator
}

// applyDefaults fills every zero-valued separator, query key, and
// OpAliases field on o with its documented default. Called once at the
// start of ParseValues so callers may pass a partially-populated
// URLParseOptions (or nil) and only override what they need.
func (o *URLParseOptions) applyDefaults() {
	if o.FilterSep == "" {
		o.FilterSep = "|"
	}
	if o.FilterTriple == "" {
		o.FilterTriple = ":"
	}
	if o.SortSep == "" {
		o.SortSep = ","
	}
	if o.RangeSep == "" {
		o.RangeSep = ","
	}
	if o.ListSep == "" {
		o.ListSep = ","
	}
	if o.SearchKey == "" {
		o.SearchKey = "search"
	}
	if o.FilterKey == "" {
		o.FilterKey = "filter"
	}
	if o.SortKey == "" {
		o.SortKey = "sort"
	}
	if o.PageKey == "" {
		o.PageKey = "page"
	}
	if o.PageSizeKey == "" {
		o.PageSizeKey = "pageSize"
	}
	if o.FieldsKey == "" {
		o.FieldsKey = "fields"
	}
	if o.PreloadsKey == "" {
		o.PreloadsKey = "preloads"
	}
	if o.OpAliases == nil {
		o.OpAliases = DefaultOpAliases
	}
}

// ParseRequest parses r.URL.Query() into a model.Query, using the grammar
// documented on the binding package. Pass nil opts to use DefaultOpAliases
// and every other default key/separator.
//
// Example, for a request to:
//
//	GET /v2/products?search=capital&sort=type_id:asc&pageSize=10&page=2&filter=type_id:1:equals|data_status:Active:equals
//
// ParseRequest(r, nil) returns a Query with Search "capital", one ascending
// Sort on "type_id", Page 2, PageSize 10, and two Filters ("type_id" IsEqual
// "1", "data_status" IsEqual "Active").
func ParseRequest(r *http.Request, opts *URLParseOptions) (model.Query, error) {
	return ParseValues(r.URL.Query(), opts)
}

// ParseRawURL parses a full URL string (e.g. "https://host/path?search=...")
// into a model.Query. It is a convenience wrapper around ParseValues for
// tests and callers that are not working with an *http.Request. Pass nil
// opts for defaults.
func ParseRawURL(raw string, opts *URLParseOptions) (model.Query, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return model.Query{}, fmt.Errorf("binding: invalid url: %w", err)
	}
	return ParseValues(u.Query(), opts)
}

// ParseValues is the core, framework-agnostic parser that ParseRequest and
// ParseRawURL both delegate to. It reads Search, Page, PageSize, Sort,
// Filter, Fields, and Preloads out of v according to opts (or the defaults,
// when opts is nil), and returns a populated model.Query.
//
// ParseValues returns an error, with no partial mutation guarantee beyond
// what has already been set on the returned Query, if Page or PageSize is
// not a valid integer, if a sort clause does not match "field:asc|desc", or
// if a filter clause does not match "field:value:op" or uses an operator
// not present in opts.OpAliases.
func ParseValues(v url.Values, opts *URLParseOptions) (model.Query, error) {
	if opts == nil {
		opts = &URLParseOptions{}
	}
	opts.applyDefaults()

	q := model.Query{
		Search: v.Get(opts.SearchKey),
	}

	if s := v.Get(opts.PageKey); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			return q, fmt.Errorf("binding: %s must be integer: %w", opts.PageKey, err)
		}
		q.SelectParameter.PageDescriptor.Page = n
	}
	if s := v.Get(opts.PageSizeKey); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			return q, fmt.Errorf("binding: %s must be integer: %w", opts.PageSizeKey, err)
		}
		q.SelectParameter.PageDescriptor.PageSize = n
	}

	if raw := v.Get(opts.SortKey); raw != "" {
		sorts, err := parseSorts(raw, opts.SortSep)
		if err != nil {
			return q, err
		}
		q.SelectParameter.Sorts = sorts
	}

	if raw := v.Get(opts.FilterKey); raw != "" {
		filters, err := parseFilters(raw, opts)
		if err != nil {
			return q, err
		}
		q.SelectParameter.Filters = filters
	}

	if raw := v.Get(opts.FieldsKey); raw != "" {
		q.SelectParameter.Fields = splitTrim(raw, opts.ListSep)
	}
	if raw := v.Get(opts.PreloadsKey); raw != "" {
		q.SelectParameter.Preloads = splitTrim(raw, opts.ListSep)
	}

	return q, nil
}

// parseSorts splits raw on sep into individual "field:asc|desc" clauses and
// converts each into a model.Sort. Direction matching is case-insensitive;
// any value other than "asc" or "desc" is an error.
func parseSorts(raw, sep string) ([]model.Sort, error) {
	var out []model.Sort
	for _, part := range strings.Split(raw, sep) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		field, dir, ok := strings.Cut(part, ":")
		if !ok {
			return nil, fmt.Errorf("binding: bad sort clause %q, want field:asc|desc", part)
		}
		s := model.Sort{FieldName: strings.TrimSpace(field)}
		switch strings.ToLower(strings.TrimSpace(dir)) {
		case "asc":
			s.SortDirection = model.Ascending
		case "desc":
			s.SortDirection = model.Descending
		default:
			return nil, fmt.Errorf("binding: bad sort direction %q in %q", dir, part)
		}
		out = append(out, s)
	}
	return out, nil
}

// parseFilters splits raw on opts.FilterSep into individual
// "field:value:op" clauses (value itself may contain opts.FilterTriple,
// since the split is limited to 3 parts) and converts each into a
// model.Filter. The operator alias is looked up case-insensitively in
// opts.OpAliases; an unrecognized alias is an error rather than a silent
// fallback. For the isin/isnotin aliases, value is split on opts.RangeSep
// into Filter.RangeValues instead of Filter.Value.
func parseFilters(raw string, opts *URLParseOptions) ([]model.Filter, error) {
	var out []model.Filter
	for _, part := range strings.Split(raw, opts.FilterSep) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		bits := strings.SplitN(part, opts.FilterTriple, 3)
		if len(bits) != 3 {
			return nil, fmt.Errorf("binding: bad filter clause %q, want field:value:op", part)
		}

		field := strings.TrimSpace(bits[0])
		rawVal := bits[1]
		rawOp := strings.ToLower(strings.TrimSpace(bits[2]))

		if field == "" {
			return nil, fmt.Errorf("binding: empty field name in %q", part)
		}

		op, ok := opts.OpAliases[rawOp]
		if !ok {
			return nil, fmt.Errorf("binding: unknown operator %q in %q", rawOp, part)
		}

		f := model.Filter{FieldName: field, Operator: op}

		switch op {
		case model.IsIn, model.IsNotIn:
			parts := splitTrim(rawVal, opts.RangeSep)
			f.RangeValues = make([]any, len(parts))
			for i, p := range parts {
				f.RangeValues[i] = p
			}
		default:
			f.Value = rawVal
		}

		out = append(out, f)
	}
	return out, nil
}

// splitTrim splits s on sep, trims surrounding whitespace from each part,
// and drops any part that is empty after trimming.
func splitTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
