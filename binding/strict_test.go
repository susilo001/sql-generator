package binding

import (
	"net/url"
	"testing"
)

func TestStrictPagination(t *testing.T) {
	for _, key := range []string{"page", "page_size"} {
		for _, value := range []string{"", "0", "-1", "1.5", "oops", "99999999999999999999"} {
			_, err := ParseValues(url.Values{key: {value}}, &URLParseOptions{StrictPagination: true, PageSizeKey: "page_size"})
			if err == nil {
				t.Fatalf("accepted %s=%q", key, value)
			}
		}
	}
}

func TestRepeatedFiltersAndPipes(t *testing.T) {
	query, err := ParseValues(url.Values{"filter": {"price:10:greaterthanorequal|price:20:lessthanorequal", "stock:0:equals"}}, nil)
	if err != nil || len(query.SelectParameter.Filters) != 3 {
		t.Fatalf("query=%+v error=%v", query, err)
	}
}
