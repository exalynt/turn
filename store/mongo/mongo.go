package mongo

import (
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/exalynt/turn/store"
)

// Find holds the parts of a MongoDB Find for one page fetch.
type Find struct {
	// Filter is the keyset filter, or empty if the query has no boundary.
	// Combine it with the consumer's own filter using [Find.And].
	Filter bson.D

	// Sort is the sort document, such as {created_at: -1, _id: -1}.
	Sort bson.D

	// Limit is the most documents to return, including the lookahead item.
	Limit int64

	// Skip is the number of documents to skip. It is zero for cursor reads.
	Skip int64
}

// Render returns the Find parts for q.
//
// The keyset filter bounds the first key, so an index on the keys still
// applies, and compares the keys one at a time. Sorting by created_at
// descending and _id ascending, it is
//
//	{created_at: {$lte: x}, $or: [
//		{created_at: {$lt: x}},
//		{created_at: {$eq: x}, _id: {$gt: y}},
//	]}
func Render(q store.Query) Find {
	f := Find{Sort: make(bson.D, len(q.Sort)), Limit: int64(q.Limit), Skip: q.Offset}
	for i, s := range q.Sort {
		dir := 1
		if s.Desc {
			dir = -1
		}
		f.Sort[i] = bson.E{Key: s.Field, Value: dir}
	}
	seek := q.Seek()
	if seek == nil {
		return f
	}
	if len(seek) == 1 {
		f.Filter = bson.D{field(seek[0][0])}
		return f
	}
	first := seek[0][0]
	op := "$lte"
	if first.Op == store.OpGreater {
		op = "$gte"
	}
	or := make(bson.A, len(seek))
	for i, conds := range seek {
		doc := make(bson.D, len(conds))
		for j, c := range conds {
			doc[j] = field(c)
		}
		or[i] = doc
	}
	f.Filter = bson.D{
		{Key: first.Field, Value: bson.D{{Key: op, Value: first.Value}}},
		{Key: "$or", Value: or},
	}
	return f
}

// And returns filter combined with the keyset filter: filter alone if there is
// no keyset filter, the keyset filter alone if filter is nil, and otherwise
// {$and: [filter, f.Filter]}. It never returns nil, so the result can be
// passed to Find as is.
func (f Find) And(filter any) any {
	switch {
	case len(f.Filter) == 0 && filter == nil:
		return bson.D{}
	case len(f.Filter) == 0:
		return filter
	case filter == nil:
		return f.Filter
	}
	return bson.D{{Key: "$and", Value: bson.A{filter, f.Filter}}}
}

// Options returns the Find options that apply Sort, Limit, and Skip.
func (f Find) Options() *options.FindOptionsBuilder {
	o := options.Find().SetSort(f.Sort).SetLimit(f.Limit)
	if f.Skip != 0 {
		o.SetSkip(f.Skip)
	}
	return o
}

// field returns c as a filter element, such as {created_at: {$lt: x}}.
func field(c store.Cond) bson.E {
	op := "$eq"
	switch c.Op {
	case store.OpLess:
		op = "$lt"
	case store.OpGreater:
		op = "$gt"
	}
	return bson.E{Key: c.Field, Value: bson.D{{Key: op, Value: c.Value}}}
}
