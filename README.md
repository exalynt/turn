# turn

[![Stability: Alpha](https://img.shields.io/badge/stability-alpha-f97316)](https://readme.exalynt.com/how-it-works/stability-levels)

A lightweight Go library for cursor and page-based pagination.

turn handles the parts of pagination every service repeats: bounding the requested page size, turning a request into what a query needs, and shaping the result. It never builds or runs queries, and it depends only on the standard library. Your code keeps ownership of query construction, filters, authorization, record mapping, and the response format.

## Stability

turn is in **Alpha**, one of Exalynt's [stability levels](https://readme.exalynt.com/how-it-works/stability-levels). It works, but it is still taking shape. Expect frequent breaking changes, sometimes without notice, and expect bugs. Build on it only to experiment, and expect its interfaces to change.

## Install

```sh
go get github.com/exalynt/turn
```

turn needs Go 1.23 or newer.

The `paginator` package holds what every paginator shares: `paginator.Page`, `paginator.Policy`, `paginator.Window`, and the errors. The paginators themselves live in `paginator/offset` and `paginator/cursor`, the cursor codec interface in the `codec` package, and the default codec in `codec/plain`:

```go
import (
    "github.com/exalynt/turn/codec"
    "github.com/exalynt/turn/codec/plain"
    "github.com/exalynt/turn/paginator"
    "github.com/exalynt/turn/paginator/cursor"
    "github.com/exalynt/turn/paginator/offset"
)
```

## How it works

turn supports two strategies:

- **Numbered pages** (`offset.Paginator`): page 1, 2, 3, … backed by `LIMIT`/`OFFSET`.
- **Cursors** (`cursor.Paginator`): keyset pagination that continues from the last item a client saw.

Both follow the same three steps:

1. **Prepare** a plan from the client's request. This validates the request and works out what your query needs.
2. **Query** your own store using the plan, fetching at most `plan.FetchLimit()` items.
3. **Finish** the plan with the fetched items to get a `paginator.Page`.

`FetchLimit()` is the page size plus one. That extra lookahead item tells `Finish` whether more items exist, so neither strategy needs a `COUNT` query. `Finish` removes it before returning the page.

```go
type Page[T, I any] struct {
    Items   []T  // at most Size items, in canonical order; never nil
    Info    I    // offset.Info or cursor.Info
    HasMore bool // whether the query found items beyond this page
}
```

Create a paginator once per listing, for example when you build a repository, and reuse it. Paginators are safe for concurrent use. Everything specific to a request lives in the plan.

The examples below list users from PostgreSQL with `database/sql`, ordered newest first:

```go
type User struct {
    ID        string
    Name      string
    CreatedAt time.Time
}
```

## Numbered pages

### Set up

```go
users, err := offset.New[User](offset.Options{})
```

The zero `offset.Options` uses the default size policy: 25 items by default and at most 100. To cap how deep clients can page, set `MaxOffset`:

```go
maxOffset := int64(10_000)

users, err := offset.New[User](offset.Options{
    Policy:    paginator.Policy{DefaultSize: 20, MaxSize: 50},
    MaxOffset: &maxOffset,
})
```

### List a page

```go
func (r *UserRepository) ListByPage(ctx context.Context, request offset.Request) (paginator.Page[User, offset.Info], error) {
    plan, err := r.offset.Prepare(request)
    if err != nil {
        return paginator.Page[User, offset.Info]{}, err
    }

    rows, err := r.db.QueryContext(ctx, `
        SELECT id, name, created_at FROM users
        ORDER BY created_at DESC, id DESC
        LIMIT $1 OFFSET $2`,
        plan.FetchLimit(), plan.Offset)
    if err != nil {
        return paginator.Page[User, offset.Info]{}, err
    }

    users, err := scanUsers(rows)
    if err != nil {
        return paginator.Page[User, offset.Info]{}, err
    }

    return r.offset.Finish(plan, users)
}
```

`offset.Request{Number: 3, Size: 25}` prepares `Offset` 50 and `FetchLimit()` 26. A zero `Number` means page 1, and a zero `Size` means the policy's default. A page past the end of the data isn't an error; it comes back empty.

### Navigate

`page.Info` holds the page's `Number` and `Size`. When `page.HasMore` is true, `Number + 1` is the next page. A `Number` above 1 means there's an earlier page to link to, though turn doesn't check whether it still has items. Keep `Size` the same between requests, since changing it moves which items each page number covers.

## Cursors

### Define a position

A position is the set of ordered values that identifies one item in your listing's canonical order. Ordering by `created_at DESC, id DESC` needs both values, because `created_at` alone isn't unique:

```go
type UserPosition struct {
    CreatedAt time.Time
    ID        string
}
```

A listing ordered by a single unique column can use that column's type as the position.

### Choose a codec

A `codec.Codec` turns positions into the opaque `codec.Cursor` strings clients hold, and back again. The default, `plain.Codec` from `github.com/exalynt/turn/codec/plain`, encodes the position as JSON behind a version byte and a digest of the scope, then as unpadded base64url, so cursors go in a URL without escaping. Its zero value is ready to use, as in the example below.

Your position type must round-trip through `encoding/json` exactly. `time.Time` does, at nanosecond precision.

`plain` cursors aren't signed: a client can read the position and forge another. To sign or encrypt cursors, or to use another format, implement `codec.Codec` yourself.

Keep these in mind when writing your own codec:

- **Version the encoding.** Clients keep cursors across your deploys, so a cursor issued by the previous release should still decode.
- **Check the scope.** `Decode` should reject cursors issued for a different scope (see below).
- **Encoding isn't protection.** Base64 hides nothing, and anyone can forge a cursor. If that matters, sign cursors in your codec, for example with HMAC. Cursor checks never replace your own authorization.
- **Preserve values exactly.** A position that loses precision on a round trip, such as a truncated timestamp, can skip or repeat items.

### Create the cursor paginator

```go
users, err := cursor.New(cursor.Options[User, UserPosition]{
    Codec: plain.Codec[UserPosition]{},
    Position: func(u User) UserPosition {
        return UserPosition{CreatedAt: u.CreatedAt, ID: u.ID}
    },
})
```

`Policy` works the same as for numbered pages. `Codec` and `Position` are required, and both must be safe for concurrent use.

### Query by cursor

`Prepare` takes a scope alongside the request. The scope names what a cursor is valid for: the resource, the filters, the caller's authorization scope, and the ordering. Your codec receives it when encoding and decoding, so a cursor from one listing can't be replayed against another.

The plan tells your query where to start and which way to read:

- **`cursor.Forward`** selects items strictly after `plan.Boundary`, in canonical order.
- **`cursor.Backward`** selects items strictly before `plan.Boundary`, in **reverse** canonical order. `Finish` flips them back.
- A nil `Boundary` (no cursor in the request) starts at the beginning for `Forward` and at the end for `Backward`.

```go
func (r *UserRepository) ListByCursor(ctx context.Context, request cursor.Request) (paginator.Page[User, cursor.Info], error) {
    plan, err := r.cursor.Prepare(request, "users:created_at_desc")
    if err != nil {
        return paginator.Page[User, cursor.Info]{}, err
    }

    // Canonical order is newest first; Backward reads the reverse.
    order, compare := "DESC", "<"
    if plan.Direction == cursor.Backward {
        order, compare = "ASC", ">"
    }

    query := "SELECT id, name, created_at FROM users"
    var args []any
    if b := plan.Boundary; b != nil {
        query += fmt.Sprintf(" WHERE (created_at, id) %s ($1, $2)", compare)
        args = append(args, b.CreatedAt, b.ID)
    }
    query += fmt.Sprintf(" ORDER BY created_at %s, id %s LIMIT $%d", order, order, len(args)+1)
    args = append(args, plan.FetchLimit())

    rows, err := r.db.QueryContext(ctx, query, args...)
    if err != nil {
        return paginator.Page[User, cursor.Info]{}, err
    }

    users, err := scanUsers(rows)
    if err != nil {
        return paginator.Page[User, cursor.Info]{}, err
    }

    return r.cursor.Finish(plan, users)
}
```

If the listing has filters, apply the same filters to the query and include them in the scope, for example `"users:status=active:created_at_desc"`.

### Navigate with cursors

`page.Info` holds `StartCursor` and `EndCursor`, which are the positions of the first and last items in canonical order. Both are empty when the page has no items.

```go
// Next page, reading forward.
if page.HasMore {
    next := cursor.Request{Cursor: page.Info.EndCursor}
}

// Previous page, reading backward.
previous := cursor.Request{Direction: cursor.Backward, Cursor: page.Info.StartCursor}
```

`HasMore` only covers the direction the page was read in. A forward page doesn't report whether earlier items exist, and a backward page doesn't report whether later ones do.

## Serving over HTTP

turn doesn't parse requests or write responses, so your handler does both. Use `errors.Is` to tell a client's bad input from your own failures:

```go
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
    var request offset.Request
    var err error
    if v := r.URL.Query().Get("page"); v != "" {
        if request.Number, err = strconv.ParseInt(v, 10, 64); err != nil {
            http.Error(w, "page must be a number", http.StatusBadRequest)
            return
        }
    }
    if v := r.URL.Query().Get("size"); v != "" {
        if request.Size, err = strconv.Atoi(v); err != nil {
            http.Error(w, "size must be a number", http.StatusBadRequest)
            return
        }
    }

    page, err := h.users.ListByPage(r.Context(), request)
    switch {
    case isBadRequest(err):
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    case err != nil:
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(map[string]any{
        "items":    page.Items,
        "page":     page.Info.Number,
        "size":     page.Info.Size,
        "has_more": page.HasMore,
    })
}

func isBadRequest(err error) bool {
    return errors.Is(err, paginator.ErrInvalidSize) ||
        errors.Is(err, paginator.ErrInvalidPage) ||
        errors.Is(err, paginator.ErrOffsetTooLarge) ||
        errors.Is(err, paginator.ErrInvalidDirection) ||
        errors.Is(err, paginator.ErrInvalidCursor)
}
```

### Errors

| Error | Cause | Typical response |
| --- | --- | --- |
| `ErrInvalidSize` | Size is negative or above `MaxSize` | 400 |
| `ErrInvalidPage` | Page number is negative | 400 |
| `ErrOffsetTooLarge` | Page is deeper than `MaxOffset` allows, or the offset overflows | 400 |
| `ErrInvalidDirection` | Direction isn't `Forward` or `Backward` | 400 |
| `ErrInvalidCursor` | The codec couldn't decode the cursor for this scope | 400 |
| `ErrInvalidOptions` | The paginator's configuration is invalid | Fix at startup |
| `ErrInvalidPlan` | `Finish` got a plan this paginator didn't prepare, or one that was changed | 500 |
| `ErrInvalidBatch` | The query returned more than `FetchLimit()` items | 500 |

`Finish` on a cursor paginator can also return an error from your codec's `Encode`.

## Things to get right

- **Use a deterministic order** that ends with a unique column, such as `id`. Without one, items with equal sort values can repeat or go missing between pages.
- **Return the whole query result to `Finish`.** Fewer than `FetchLimit()` items tells turn the listing is exhausted. If the query fails, return its error rather than finishing a partial batch.
- **Don't change plans.** Pass the plan from `Prepare` to `Finish` as it is. `Finish` rejects plans it couldn't have produced.
- **Neither strategy is a snapshot.** Inserts and deletes between requests shift numbered pages, and updates to sort columns can move items across cursor boundaries. Transactions and snapshots are up to you.
- **Deep offsets are slow.** The database still reads every skipped row. Set `MaxOffset`, or use cursors for large or unbounded listings.

`Finish` never reorders or modifies the slice you pass in, and appending to `page.Items` never overwrites it.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
