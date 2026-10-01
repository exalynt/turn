package http

import (
	"net/url"
	"strings"
)

// Relation types for navigating a paginated listing, registered in the IANA
// link relations registry.
const (
	RelFirst = "first"
	RelPrev  = "prev"
	RelNext  = "next"
	RelLast  = "last"
)

// Link is a link to a target URL with a relation type, such as [RelNext].
type Link struct {
	// Rel is the relation type: a registered name, such as [RelNext], or an
	// absolute URI. Several types may be separated by spaces.
	Rel string

	// URL is the link target. It may be relative, in which case clients
	// resolve it against the request URL. A nil URL omits the link.
	URL *url.URL
}

// FormatLinks returns links as one Link header value, in the order given,
// such as `</users?page=2>; rel="next", </users?page=1>; rel="first"`. Links
// with a nil URL are omitted, so a link to a page that may not exist can be
// passed unconditionally. FormatLinks returns the empty string if no link
// remains; don't set the header in that case.
func FormatLinks(links ...Link) string {
	var b strings.Builder
	for _, l := range links {
		if l.URL == nil {
			continue
		}
		if b.Len() > 0 {
			b.WriteString(", ")
		}
		b.WriteByte('<')
		b.WriteString(target.Replace(l.URL.String()))
		b.WriteString(`>; rel="`)
		b.WriteString(quoted.Replace(l.Rel))
		b.WriteByte('"')
	}
	return b.String()
}

var (
	// target percent-encodes characters a URL can carry through a raw query
	// or opaque part but that may not appear between the angle brackets.
	target = strings.NewReplacer("<", "%3C", ">", "%3E", `"`, "%22", " ", "%20")

	// quoted escapes a quoted-string's delimiters.
	quoted = strings.NewReplacer(`\`, `\\`, `"`, `\"`)
)
