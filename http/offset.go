package http

import (
	"math"
	"net/url"
	"strconv"

	"github.com/exalynt/turn/paginator"
	"github.com/exalynt/turn/paginator/offset"
)

// OffsetQuery reads and writes numbered-page selectors as URL query
// parameters, such as ?page=3&size=25. The zero OffsetQuery uses the
// parameters page and size, and is safe for concurrent use.
type OffsetQuery struct {
	// Page and Size name the query parameters. Empty names select [ParamPage]
	// and [ParamSize].
	Page, Size Param
}

// Parse returns the selector held by values, typically r.URL.Query(). An
// absent or empty parameter leaves its field zero, selecting page 1 or the
// policy's default size.
//
// It returns an error wrapping [paginator.ErrInvalidPage] if the page is not
// an integer and [paginator.ErrInvalidSize] if the size is not. Parse does not
// check either against the paginator's limits; the paginator's Prepare does.
func (q OffsetQuery) Parse(values url.Values) (offset.Selector, error) {
	page, size := q.params()
	number, err := parseInt(values, page, 64, paginator.ErrInvalidPage)
	if err != nil {
		return offset.Selector{}, err
	}
	n, err := parseInt(values, size, strconv.IntSize, paginator.ErrInvalidSize)
	if err != nil {
		return offset.Selector{}, err
	}
	return offset.Selector{Number: number, Size: int(n)}, nil
}

// URL returns a function that returns the URL selecting a page: base with the
// page and size parameters set, keeping its other parameters, such as filters.
// A zero Number or Size leaves that parameter out. It suits [OffsetLinks],
// with the current request URL as base.
func (q OffsetQuery) URL(base *url.URL) func(offset.Selector) *url.URL {
	page, size := q.params()
	return func(selector offset.Selector) *url.URL {
		return withQuery(base, func(values url.Values) {
			values.Del(page)
			values.Del(size)
			if selector.Number != 0 {
				values.Set(page, strconv.FormatInt(selector.Number, 10))
			}
			if selector.Size != 0 {
				values.Set(size, strconv.Itoa(selector.Size))
			}
		})
	}
}

// params returns the parameter names, with defaults for empty fields.
func (q OffsetQuery) params() (page, size string) {
	return orDefault(q.Page, ParamPage), orDefault(q.Size, ParamSize)
}

// OffsetLinks returns the links next to a numbered page, for [FormatLinks]:
//
//   - [RelFirst]: always; page 1.
//   - [RelPrev]: the previous page, when the page number is above 1.
//   - [RelNext]: the next page, when page.HasMore is true.
//
// There is no [RelLast] link, since turn does not count items. url returns the
// URL selecting a page, such as one from [OffsetQuery.URL]. Every selector it
// receives has the page's Size, since page numbers cover the same items only
// at the same size. A nil URL leaves that link out.
func OffsetLinks[T any](page offset.Page[T], url func(offset.Selector) *url.URL) []Link {
	at := func(rel string, number int64) Link {
		return Link{Rel: rel, URL: url(offset.Selector{Number: number, Size: page.Size})}
	}
	links := []Link{at(RelFirst, 1)}
	if page.Number > 1 {
		links = append(links, at(RelPrev, page.Number-1))
	}
	if page.HasMore && page.Number < math.MaxInt64 {
		links = append(links, at(RelNext, page.Number+1))
	}
	return links
}
