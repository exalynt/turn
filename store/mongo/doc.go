// Package mongo renders a [store.Query] for MongoDB's Find: a keyset filter,
// a sort, a limit, and a skip. The consumer combines the filter with its own
// and runs the query, so the package never touches a client or collection.
//
//	f := turnmongo.Render(q)
//	cur, err := users.Find(ctx, f.And(bson.D{{Key: "status", Value: "active"}}), f.Options())
//
// Sort fields are document field paths, emitted verbatim. They must come from
// the consumer's code, never from request input. Keyset values must not be
// null, and must encode to the same BSON type as the stored field; a position
// holding an ID as a string does not match a field holding a
// [bson.ObjectID], so keep the ObjectID in the position.
//
// This package is its own module, github.com/exalynt/turn/store/mongo, so only
// consumers that import it depend on the MongoDB driver. It shares its name
// with the driver's mongo package, so import it under another name, such as
// turnmongo, in files that use both.
package mongo
