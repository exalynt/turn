package sql

import (
	"strings"

	"github.com/exalynt/turn/store"
)

// Clause holds the SQL fragments for one page fetch.
type Clause struct {
	// Where is the keyset predicate, without the WHERE keyword, or empty if
	// the query has no boundary.
	Where string

	// OrderBy is the ORDER BY clause, or empty if the query has no sort.
	OrderBy string

	// Limit is the limit clause, such as "LIMIT $3 OFFSET $4".
	Limit string

	// Args holds the arguments of Where and then Limit, in placeholder order.
	Args []any
}

// And returns Where as a condition to append to a query's own WHERE clause,
// " AND (<Where>)", or the empty string if there is no predicate.
func (c Clause) And() string {
	if c.Where == "" {
		return ""
	}
	return " AND (" + c.Where + ")"
}

// Filter returns Where as a WHERE clause, " WHERE <Where>", for a query
// without one of its own, or the empty string if there is no predicate.
func (c Clause) Filter() string {
	if c.Where == "" {
		return ""
	}
	return " WHERE " + c.Where
}

// Tail returns the ORDER BY and limit clauses to end a query with, each
// preceded by a space.
func (c Clause) Tail() string {
	var b strings.Builder
	for _, part := range []string{c.OrderBy, c.Limit} {
		if part != "" {
			b.WriteByte(' ')
			b.WriteString(part)
		}
	}
	return b.String()
}

// Render returns the fragments for q in dialect d. Placeholders are numbered
// from start, so the fragments can follow the consumer's own arguments; a start
// below 1 selects 1. Each value gets its own placeholder.
//
// When every key sorts the same direction and d supports row values, the
// predicate compares the boundary as one row:
//
//	(created_at, id) < ($1, $2)
//
// Otherwise it bounds the first key, so an index on the keys still applies,
// and compares the keys one at a time:
//
//	created_at <= $1 AND (created_at < $2 OR (created_at = $3 AND id > $4))
func Render(d Dialect, q store.Query, start int) Clause {
	r := renderer{dialect: d, next: max(start, 1)}
	c := Clause{Where: r.where(q), OrderBy: orderBy(q.Sort)}
	limit, offset := r.arg(q.Limit), ""
	if q.Offset != 0 {
		offset = r.arg(q.Offset)
	}
	c.Limit = d.Limit(limit, offset)
	c.Args = r.args
	return c
}

// renderer collects arguments while rendering placeholders.
type renderer struct {
	dialect Dialect
	next    int
	args    []any
}

// arg adds v to the arguments and returns its placeholder.
func (r *renderer) arg(v any) string {
	r.args = append(r.args, v)
	p := r.dialect.Placeholder(r.next)
	r.next++
	return p
}

// where returns the keyset predicate for q, or the empty string if q has no
// boundary.
func (r *renderer) where(q store.Query) string {
	seek := q.Seek()
	switch {
	case seek == nil:
		return ""
	case len(q.Sort) == 1:
		return r.cond(seek[0][0])
	case r.dialect.RowValues() && uniform(q.Sort):
		return r.row(q)
	}
	// Bound the first key inclusively, then require one of the disjuncts.
	first := seek[0][0]
	op := "<="
	if first.Op == store.OpGreater {
		op = ">="
	}
	var b strings.Builder
	b.WriteString(first.Field + " " + op + " " + r.arg(first.Value) + " AND (")
	for i, conds := range seek {
		if i > 0 {
			b.WriteString(" OR ")
		}
		if len(conds) > 1 {
			b.WriteByte('(')
		}
		for j, cond := range conds {
			if j > 0 {
				b.WriteString(" AND ")
			}
			b.WriteString(r.cond(cond))
		}
		if len(conds) > 1 {
			b.WriteByte(')')
		}
	}
	b.WriteByte(')')
	return b.String()
}

// row returns q's boundary as a row value comparison. Every key of q must
// sort the same direction.
func (r *renderer) row(q store.Query) string {
	fields := make([]string, len(q.Sort))
	values := make([]string, len(q.Sort))
	for i, s := range q.Sort {
		fields[i] = s.Field
		values[i] = r.arg(q.After[i])
	}
	op := ">"
	if q.Sort[0].Desc {
		op = "<"
	}
	return "(" + strings.Join(fields, ", ") + ") " + op + " (" + strings.Join(values, ", ") + ")"
}

// cond returns c as a comparison.
func (r *renderer) cond(c store.Cond) string {
	op := "="
	switch c.Op {
	case store.OpLess:
		op = "<"
	case store.OpGreater:
		op = ">"
	}
	return c.Field + " " + op + " " + r.arg(c.Value)
}

// orderBy returns the ORDER BY clause for order, or the empty string if order
// is empty.
func orderBy(order []store.Sort) string {
	if len(order) == 0 {
		return ""
	}
	keys := make([]string, len(order))
	for i, s := range order {
		keys[i] = s.Field + " ASC"
		if s.Desc {
			keys[i] = s.Field + " DESC"
		}
	}
	return "ORDER BY " + strings.Join(keys, ", ")
}

// uniform reports whether every key of order sorts the same direction.
func uniform(order []store.Sort) bool {
	for _, s := range order[1:] {
		if s.Desc != order[0].Desc {
			return false
		}
	}
	return true
}
