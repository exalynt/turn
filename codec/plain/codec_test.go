package plain_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"testing"
	"time"

	"github.com/exalynt/turn/codec"
	"github.com/exalynt/turn/codec/plain"
)

type position struct {
	CreatedAt time.Time
	ID        string
}

func TestRoundTrip(t *testing.T) {
	c := plain.Codec[position]{}
	want := position{
		CreatedAt: time.Date(2026, 3, 14, 15, 9, 26, 535897932, time.FixedZone("UTC+2", 2*60*60)),
		ID:        "user-42",
	}
	cursor, err := c.Encode(want, "users")
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Decode(cursor, "users")
	if err != nil {
		t.Fatalf("Decode(%q) error = %v", cursor, err)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) || got.ID != want.ID {
		t.Errorf("Decode(Encode(%+v)) = %+v", want, got)
	}
}

func TestRoundTripScalars(t *testing.T) {
	ints := plain.Codec[int64]{}
	for _, want := range []int64{0, -1, math.MaxInt64, math.MinInt64} {
		cursor, err := ints.Encode(want, "")
		if err != nil {
			t.Fatal(err)
		}
		if got, err := ints.Decode(cursor, ""); err != nil || got != want {
			t.Errorf("Decode(Encode(%d)) = %d, %v", want, got, err)
		}
	}

	strs := plain.Codec[string]{}
	for _, want := range []string{"", "héllo wörld", "a/b+c=d?e&f", "\x00"} {
		cursor, err := strs.Encode(want, "scope")
		if err != nil {
			t.Fatal(err)
		}
		if got, err := strs.Decode(cursor, "scope"); err != nil || got != want {
			t.Errorf("Decode(Encode(%q)) = %q, %v", want, got, err)
		}
	}
}

// TestCursorFormatStable guards cursors already handed to clients: a cursor
// issued by an earlier release must keep decoding.
func TestCursorFormatStable(t *testing.T) {
	const cursor codec.Cursor = "AaM7O-U2Mr8deyJDcmVhdGVkQXQiOiIyMDI2LTAxLTAyVDAzOjA0OjA1LjAwMDAwMDAwNloiLCJJRCI6InUxIn0"
	c := plain.Codec[position]{}
	want := position{CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC), ID: "u1"}

	got, err := c.Decode(cursor, "users:created_at_desc")
	if err != nil {
		t.Fatalf("Decode(%q) error = %v", cursor, err)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) || got.ID != want.ID {
		t.Errorf("Decode(%q) = %+v, want %+v", cursor, got, want)
	}
	if encoded, err := c.Encode(want, "users:created_at_desc"); err != nil || encoded != cursor {
		t.Errorf("Encode(%+v) = %q, %v, want %q", want, encoded, err, cursor)
	}
}

func TestEncodeURLSafe(t *testing.T) {
	c := plain.Codec[string]{}
	// Varied inputs make the base64 output cover the URL-unsafe standard
	// alphabet characters if the codec used them.
	for _, s := range []string{"", "?", ">>>", "???", "~~~~", "\xff\xfe\xfd", "a long position value"} {
		cursor, err := c.Encode(s, "scope")
		if err != nil {
			t.Fatal(err)
		}
		if url.QueryEscape(string(cursor)) != string(cursor) {
			t.Errorf("Encode(%q) = %q, which needs URL escaping", s, cursor)
		}
	}
}

func TestEncodeDeterministic(t *testing.T) {
	c := plain.Codec[int]{}
	a, _ := c.Encode(7, "scope")
	b, _ := c.Encode(7, "scope")
	if a != b {
		t.Errorf("Encode(7) twice = %q and %q, want equal", a, b)
	}
	if other, _ := c.Encode(7, "other"); other == a {
		t.Errorf("Encode(7) in two scopes = %q for both, want different cursors", a)
	}
}

func TestEncodeError(t *testing.T) {
	cursor, err := plain.Codec[chan int]{}.Encode(make(chan int), "scope")
	var unsupported *json.UnsupportedTypeError
	if !errors.As(err, &unsupported) {
		t.Fatalf("Encode(chan) error = %v, want *json.UnsupportedTypeError", err)
	}
	if cursor != "" {
		t.Errorf("Encode(chan) cursor = %q, want empty", cursor)
	}
}

func TestDecodeScopeMismatch(t *testing.T) {
	c := plain.Codec[position]{}
	cursor, err := c.Encode(position{ID: "u1"}, "users")
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Decode(cursor, "users:status=active")
	if !errors.Is(err, plain.ErrScopeMismatch) {
		t.Fatalf("Decode in another scope error = %v, want %v", err, plain.ErrScopeMismatch)
	}
	if got != (position{}) {
		t.Errorf("Decode in another scope = %+v, want zero position", got)
	}
}

func TestDecodeErrors(t *testing.T) {
	c := plain.Codec[position]{}
	valid, err := c.Encode(position{ID: "u1"}, "scope")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(string(valid))
	if err != nil {
		t.Fatal(err)
	}
	const headerSize = 9
	header := raw[:headerSize]

	// withBytes encodes data as a cursor.
	withBytes := func(data []byte) codec.Cursor {
		return codec.Cursor(base64.RawURLEncoding.EncodeToString(data))
	}
	// withBody keeps the valid header and replaces the JSON body.
	withBody := func(body string) codec.Cursor {
		return withBytes(append(append([]byte{}, header...), body...))
	}
	// withVersion replaces the version byte.
	withVersion := func(v byte) codec.Cursor {
		data := append([]byte{}, raw...)
		data[0] = v
		return withBytes(data)
	}

	tests := []struct {
		name    string
		cursor  codec.Cursor
		wantErr error
	}{
		{name: "empty", cursor: "", wantErr: plain.ErrMalformed},
		{name: "not base64", cursor: "!!!!", wantErr: plain.ErrMalformed},
		{name: "padded", cursor: valid + "==", wantErr: plain.ErrMalformed},
		{name: "standard alphabet", cursor: "ab+/", wantErr: plain.ErrMalformed},
		{name: "shorter than header", cursor: withBytes(header[:headerSize-1]), wantErr: plain.ErrMalformed},
		{name: "header only", cursor: withBytes(header), wantErr: plain.ErrMalformed},
		{name: "invalid JSON", cursor: withBody("{"), wantErr: plain.ErrMalformed},
		{name: "wrong JSON type", cursor: withBody(`"u1"`), wantErr: plain.ErrMalformed},
		{name: "partly decodable JSON", cursor: withBody(`{"ID":"u1","CreatedAt":5}`), wantErr: plain.ErrMalformed},
		{name: "version zero", cursor: withVersion(0), wantErr: plain.ErrUnsupportedVersion},
		{name: "future version", cursor: withVersion(2), wantErr: plain.ErrUnsupportedVersion},
		{name: "altered digest", cursor: withBytes(append([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0}, raw[headerSize:]...)), wantErr: plain.ErrScopeMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.Decode(tt.cursor, "scope")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Decode(%q) error = %v, want %v", tt.cursor, err, tt.wantErr)
			}
			if got != (position{}) {
				t.Errorf("Decode(%q) = %+v, want zero position", tt.cursor, got)
			}
		})
	}
}
