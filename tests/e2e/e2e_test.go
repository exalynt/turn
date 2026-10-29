//go:build e2e

package e2e

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/exalynt/turn/paginator/cursor"
)

// pageSize is the size every test requests.
const pageSize = 4

// user is the item every test lists.
type user struct {
	ID        string    `bson:"_id" json:"id"`
	Name      string    `bson:"name" json:"name"`
	Status    string    `bson:"status" json:"status"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
}

var epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// users are the fixture rows. Every listing orders them newest first, by
// created_at then id, both descending. u02 and u03 share a created_at, as do
// u06 and u07, so the id breaks the tie.
var users = []user{
	{ID: "u01", Name: "Ada", Status: "active", CreatedAt: epoch},
	{ID: "u02", Name: "Brian", Status: "active", CreatedAt: epoch.Add(1 * time.Hour)},
	{ID: "u03", Name: "Chen", Status: "active", CreatedAt: epoch.Add(1 * time.Hour)},
	{ID: "u04", Name: "Dana", Status: "inactive", CreatedAt: epoch.Add(2 * time.Hour)},
	{ID: "u05", Name: "Eli", Status: "active", CreatedAt: epoch.Add(3 * time.Hour)},
	{ID: "u06", Name: "Fatima", Status: "active", CreatedAt: epoch.Add(4 * time.Hour)},
	{ID: "u07", Name: "Gus", Status: "inactive", CreatedAt: epoch.Add(4 * time.Hour)},
	{ID: "u08", Name: "Hana", Status: "active", CreatedAt: epoch.Add(5 * time.Hour)},
	{ID: "u09", Name: "Ivan", Status: "active", CreatedAt: epoch.Add(6 * time.Hour)},
}

// env returns the environment variable key, or fallback if it is unset.
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// uniqueName returns prefix followed by a random suffix, for a table or
// collection only this test uses.
func uniqueName(t *testing.T, prefix string) string {
	t.Helper()
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return prefix + "_" + hex.EncodeToString(b)
}

// ids returns the IDs of items, in order.
func ids(items []user) []string {
	out := make([]string, len(items))
	for i, u := range items {
		out[i] = u.ID
	}
	return out
}

// readCursors pages through a cursor listing the way a client does: forward
// from the first page, following each EndCursor until HasMore is false, then
// backward from the last page, following each StartCursor until HasMore is
// false again. It returns the IDs on each page, in the order they were read.
func readCursors(t *testing.T, list func(cursor.Selector) (cursor.Page[user], error)) (forward, backward [][]string) {
	t.Helper()
	read := func(selector cursor.Selector) cursor.Page[user] {
		t.Helper()
		page, err := list(selector)
		if err != nil {
			t.Fatalf("list %+v: %v", selector, err)
		}
		if len(forward)+len(backward) > len(users) {
			t.Fatal("paging did not end")
		}
		return page
	}

	page := read(cursor.Selector{Size: pageSize})
	forward = append(forward, ids(page.Items))
	for page.HasMore {
		page = read(cursor.Selector{Size: pageSize, Cursor: page.EndCursor})
		forward = append(forward, ids(page.Items))
	}

	page = read(cursor.Selector{Direction: cursor.Backward, Size: pageSize, Cursor: page.StartCursor})
	backward = append(backward, ids(page.Items))
	for page.HasMore {
		page = read(cursor.Selector{Direction: cursor.Backward, Size: pageSize, Cursor: page.StartCursor})
		backward = append(backward, ids(page.Items))
	}
	return forward, backward
}

// assertPages fails the test if got isn't want.
func assertPages(t *testing.T, name string, got, want [][]string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s pages:\n got %v\nwant %v", name, got, want)
	}
}
