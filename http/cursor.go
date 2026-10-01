package http

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/exalynt/turn/codec"
	"github.com/exalynt/turn/paginator"
	"github.com/exalynt/turn/paginator/cursor"
)

// CursorQuery reads and writes cursor selectors as URL query parameters:
// ?after=<cursor> reads forward from a cursor, ?before=<cursor> reads
// backward from one, and ?size=<n> sets the page size. Without either cursor
// a selector reads forward from the start of the listing; an empty ?before=
// reads backward from the end. The zero CursorQuery uses the parameters after,
// before, and size, and is safe for concurrent use.
type CursorQuery struct {
	// After, Before, and Size name the query parameters. Empty names select
	// [ParamAfter], [ParamBefore], and [ParamSize].
	After, Before, Size Param
}

// Parse returns the selector held by values, typically r.URL.Query(). An
// absent or empty size leaves Size zero, selecting the policy's default.
//
// It returns an error wrapping [paginator.ErrInvalidDirection] if both the
// after and before parameters are present, and [paginator.ErrInvalidSize] if
// the size is not an integer. Parse does not check the size against the
// policy or decode the cursor; the paginator's Prepare does.
func (q CursorQuery) Parse(values url.Values) (cursor.Selector, error) {
	after, before, size := q.params()
	if values.Has(after) && values.Has(before) {
		return cursor.Selector{}, fmt.Errorf("%w: both %s and %s are set", paginator.ErrInvalidDirection, after, before)
	}
	n, err := parseInt(values, size, strconv.IntSize, paginator.ErrInvalidSize)
	if err != nil {
		return cursor.Selector{}, err
	}
	selector := cursor.Selector{Size: int(n)}
	if values.Has(before) {
		selector.Direction = cursor.Backward
		selector.Cursor = codec.Cursor(values.Get(before))
	} else {
		selector.Cursor = codec.Cursor(values.Get(after))
	}
	return selector, nil
}

// URL returns a function that returns the URL selecting a page: base with the
// after, before, and size parameters set, keeping its other parameters, such
// as filters. A Backward selector always sets before, empty when reading from
// the end; a Forward selector sets after only with a cursor. A zero Size
// leaves size out. It suits [CursorLinks], with the current request URL as
// base.
func (q CursorQuery) URL(base *url.URL) func(cursor.Selector) *url.URL {
	after, before, size := q.params()
	return func(selector cursor.Selector) *url.URL {
		return withQuery(base, func(values url.Values) {
			values.Del(after)
			values.Del(before)
			values.Del(size)
			switch {
			case selector.Direction == cursor.Backward:
				values.Set(before, string(selector.Cursor))
			case selector.Cursor != "":
				values.Set(after, string(selector.Cursor))
			}
			if selector.Size != 0 {
				values.Set(size, strconv.Itoa(selector.Size))
			}
		})
	}
}

// params returns the parameter names, with defaults for empty fields.
func (q CursorQuery) params() (after, before, size string) {
	return orDefault(q.After, ParamAfter), orDefault(q.Before, ParamBefore), orDefault(q.Size, ParamSize)
}

// CursorLinks returns the links next to a cursor page, for [FormatLinks]:
//
//   - [RelFirst]: always; reading forward from the start.
//   - [RelPrev]: reading backward from the page's StartCursor, when earlier
//     items are known to exist.
//   - [RelNext]: reading forward from the page's EndCursor, when later items
//     are known to exist.
//   - [RelLast]: always; reading backward from the end.
//
// A page reports HasMore only in the direction it was read. In the other
// direction, CursorLinks links when the page was read from a cursor, since
// items existed past that boundary. An empty page has no prev or next link.
//
// url returns the URL selecting a page, such as one from [CursorQuery.URL].
// Every selector it receives has the page's Size. A nil URL leaves that link
// out, for example a last link when the API cannot express reading backward
// from the end.
func CursorLinks[T any](page paginator.Page[T, cursor.Info], url func(cursor.Selector) *url.URL) []Link {
	info := page.Info
	at := func(rel string, direction cursor.Direction, from codec.Cursor) Link {
		return Link{Rel: rel, URL: url(cursor.Selector{Direction: direction, Size: info.Size, Cursor: from})}
	}
	hasPrev, hasNext := info.Cursor != "", page.HasMore
	if info.Direction == cursor.Backward {
		hasPrev, hasNext = page.HasMore, info.Cursor != ""
	}
	links := []Link{at(RelFirst, cursor.Forward, "")}
	if hasPrev && info.StartCursor != "" {
		links = append(links, at(RelPrev, cursor.Backward, info.StartCursor))
	}
	if hasNext && info.EndCursor != "" {
		links = append(links, at(RelNext, cursor.Forward, info.EndCursor))
	}
	return append(links, at(RelLast, cursor.Backward, ""))
}
