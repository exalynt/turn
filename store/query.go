package store

// Sort is one key of a listing's sort order.
type Sort struct {
	// Field names what to sort by: a column, an expression, or a document
	// field path, as the store spells it. Adapters emit it verbatim, so it
	// must come from the consumer's code, never from request input.
	Field string

	// Desc sorts in descending order.
	Desc bool
}

// Query describes one page fetch, independent of any store. Adapters render
// it into the clauses and arguments their database needs.
type Query struct {
	// Sort is the order to read in. For a backward cursor read it is the
	// reverse of the listing's canonical order.
	Sort []Sort

	// After is the keyset boundary: one value per Sort key. The query selects
	// only items strictly after this tuple in Sort order. A nil After selects
	// from the start. Values are never nil.
	After []any

	// Limit is the most items to fetch, including the lookahead item.
	Limit int

	// Offset is the number of items to skip. It is zero for cursor reads.
	Offset int64
}

// Op is a comparison in a [Cond].
type Op uint8

const (
	// OpEqual matches values equal to the condition's value.
	OpEqual Op = iota

	// OpLess matches values less than the condition's value.
	OpLess

	// OpGreater matches values greater than the condition's value.
	OpGreater
)

// Cond compares a field with a value.
type Cond struct {
	Field string
	Op    Op
	Value any
}

// Seek returns q's keyset boundary as a disjunction of conjunctions, for
// stores that cannot compare row values. An item is after the boundary if it
// matches every condition of any one conjunction. For sort keys a ascending
// and b descending, Seek returns
//
//	[[a > x], [a = x, b < y]]
//
// Seek returns nil if q has no boundary or no sort keys.
func (q Query) Seek() [][]Cond {
	if q.After == nil || len(q.Sort) == 0 {
		return nil
	}
	seek := make([][]Cond, len(q.Sort))
	for i, s := range q.Sort {
		conds := make([]Cond, 0, i+1)
		for j := range i {
			conds = append(conds, Cond{Field: q.Sort[j].Field, Op: OpEqual, Value: q.After[j]})
		}
		conds = append(conds, Cond{Field: s.Field, Op: s.after(), Value: q.After[i]})
		seek[i] = conds
	}
	return seek
}

// after returns the comparison that selects values after a boundary value in
// s's order.
func (s Sort) after() Op {
	if s.Desc {
		return OpLess
	}
	return OpGreater
}
