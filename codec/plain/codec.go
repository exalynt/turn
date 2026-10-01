// Package plain provides turn's default [codec.Codec], which neither signs nor
// encrypts cursors. Cursors are unpadded base64url strings, safe to put in a
// URL path or query without escaping.
//
// A cursor holds a version byte, a digest of the scope it was issued for, and
// the position encoded as JSON. The scope itself is not stored, so cursors
// stay short and do not reveal it.
//
// Cursors are not encrypted or signed: a client can read the position and
// forge a cursor for any position within a scope it knows. Wrap or replace
// this codec if that matters.
package plain

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/exalynt/turn/codec"
)

// Errors reported by [Codec.Decode]. The cursor paginator wraps them with
// paginator.ErrInvalidCursor.
var (
	// ErrMalformed reports a cursor that is not valid base64url or whose
	// contents do not decode into a position.
	ErrMalformed = errors.New("plain: malformed cursor")

	// ErrUnsupportedVersion reports a cursor encoded with a version this
	// codec does not read.
	ErrUnsupportedVersion = errors.New("plain: unsupported cursor version")

	// ErrScopeMismatch reports a cursor issued for a different scope.
	ErrScopeMismatch = errors.New("plain: cursor issued for a different scope")
)

const (
	// version identifies the cursor layout.
	version byte = 1

	// digestSize is the number of scope digest bytes kept in a cursor. It is
	// enough to tell scopes apart, not to authenticate them.
	digestSize = 8

	// headerSize is the length of the version byte and scope digest.
	headerSize = 1 + digestSize
)

var encoding = base64.RawURLEncoding

// Codec encodes positions of type P as URL-safe cursors. P must round-trip
// through encoding/json without losing precision. The zero Codec is ready to
// use and safe for concurrent use.
type Codec[P any] struct{}

var _ codec.Codec[struct{}] = Codec[struct{}]{}

// Encode returns the cursor for position in scope. It returns an error if
// position cannot be encoded as JSON.
func (Codec[P]) Encode(position P, scope string) (codec.Cursor, error) {
	body, err := json.Marshal(position)
	if err != nil {
		return "", fmt.Errorf("plain: encode position: %w", err)
	}
	data := make([]byte, 0, headerSize+len(body))
	data = append(data, version)
	data = append(data, digest(scope)...)
	data = append(data, body...)
	return codec.Cursor(encoding.EncodeToString(data)), nil
}

// Decode returns the position held by cursor. It returns an error wrapping
// [ErrMalformed], [ErrUnsupportedVersion], or [ErrScopeMismatch] if cursor
// was not encoded by this codec for scope.
func (Codec[P]) Decode(cursor codec.Cursor, scope string) (P, error) {
	var position P
	data, err := encoding.DecodeString(string(cursor))
	if err != nil {
		return position, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	if len(data) < headerSize {
		return position, fmt.Errorf("%w: %d bytes", ErrMalformed, len(data))
	}
	if data[0] != version {
		return position, fmt.Errorf("%w: %d", ErrUnsupportedVersion, data[0])
	}
	if !bytes.Equal(data[1:headerSize], digest(scope)) {
		return position, ErrScopeMismatch
	}
	if err := json.Unmarshal(data[headerSize:], &position); err != nil {
		var zero P
		return zero, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	return position, nil
}

// digest returns the scope digest stored in a cursor.
func digest(scope string) []byte {
	sum := sha256.Sum256([]byte(scope))
	return sum[:digestSize]
}
