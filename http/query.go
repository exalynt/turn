package http

import (
	"fmt"
	"net/url"
	"strconv"
)

// Param is the name of a URL query parameter.
type Param string

// Default query parameter names, used by [OffsetQuery] and [CursorQuery] when
// a field is empty.
const (
	ParamPage   Param = "page"
	ParamSize   Param = "size"
	ParamAfter  Param = "after"
	ParamBefore Param = "before"
)

// parseInt returns the integer parameter name in values, or zero if it is
// absent or empty. It returns an error wrapping sentinel if the value is not
// an integer that fits in bits.
func parseInt(values url.Values, name string, bits int, sentinel error) (int64, error) {
	v := values.Get(name)
	if v == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(v, 10, bits)
	if err != nil {
		return 0, fmt.Errorf("%w: %s %q is not an integer", sentinel, name, v)
	}
	return n, nil
}

// orDefault returns name, or fallback if name is empty.
func orDefault(name, fallback Param) string {
	if name == "" {
		return string(fallback)
	}
	return string(name)
}

// withQuery returns a copy of base whose query is base's query changed by
// edit.
func withQuery(base *url.URL, edit func(values url.Values)) *url.URL {
	values := base.Query()
	edit(values)
	u := *base
	u.RawQuery = values.Encode()
	return &u
}
