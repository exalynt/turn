// Package codec defines how turn converts cursor positions to the opaque
// [Cursor] values handed to clients and back. A [Codec] is supplied to
// [github.com/exalynt/turn/strategy.CursorPaginator].
package codec

// Cursor is an opaque, encoded position handed to clients for continuing a
// listing. The empty Cursor means no position.
type Cursor string

// Codec converts positions to cursors and back. The scope identifies what a
// cursor is valid for, such as the resource, filters, authorization scope,
// and ordering; Decode should reject a cursor encoded for a different scope.
//
// Encoding alone neither hides nor protects a position. Use a codec that signs
// cursors if clients must not be able to forge them, and version the encoding
// so cursors issued before an upgrade still decode. A codec shared by a
// paginator must be safe for concurrent use.
type Codec[P any] interface {
	Encode(position P, scope string) (Cursor, error)
	Decode(cursor Cursor, scope string) (P, error)
}
