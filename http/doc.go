// Package http is turn's optional HTTP layer. It reads selectors from URL query
// parameters and builds RFC 8288 Link header values that point clients at
// adjacent pages:
//
//	Link: </users?page=1&size=25>; rel="first", </users?page=3&size=25>; rel="next"
//
// [OffsetQuery] and [CursorQuery] parse a selector from a URL query and build
// the URL that selects a page. [OffsetLinks] and [CursorLinks] work out which
// pages to link to from a finished page, asking a URL function for each one,
// and [FormatLinks] turns the links into a header value:
//
//	var q turnhttp.OffsetQuery
//
//	selector, err := q.Parse(r.URL.Query())
//	// handle err, then list the page
//	w.Header().Set("Link", turnhttp.FormatLinks(turnhttp.OffsetLinks(page, q.URL(r.URL))...))
//
// This package never writes responses; the status, headers, and body stay
// with the consumer. A service that carries pagination differently, such as
// in a request body, builds an offset.Selector or cursor.Selector itself and
// passes its own URL function to the links builders.
//
// The package shares its name with net/http, so import it under another name,
// such as turnhttp, in files that use both.
package http
