# turn

[![Stability: Alpha](https://img.shields.io/badge/stability-alpha-f97316)](https://readme.exalynt.com/how-it-works/stability-levels)

A lightweight Go library for cursor and page-based pagination.

turn handles the parts of pagination every service repeats: bounding the requested page size, turning a page selection into what a query needs, and shaping the result. Optional adapters render the pagination parts of a query, such as the keyset predicate and `ORDER BY`, for PostgreSQL, MySQL, and MongoDB. turn never runs queries, and its core module depends only on the standard library. Your code keeps ownership of the base query, filters, authorization, execution, record mapping, and the response format.

## Stability

turn is in **Alpha**, one of Exalynt's [stability levels](https://readme.exalynt.com/how-it-works/stability-levels). It works, but it is still taking shape. Expect frequent breaking changes, sometimes without notice, and expect bugs. Build on it only to experiment, and expect its interfaces to change.

## Install

```sh
go get github.com/exalynt/turn
```

turn needs Go 1.23 or newer.

The `paginator` package holds what every paginator shares: `paginator.Page`, `paginator.Policy`, `paginator.Window`, and the errors. The paginators themselves live in `paginator/offset` and `paginator/cursor`, the cursor codec interface in the `codec` package, the default codec in `codec/plain`, the optional query layer in `store`, the SQL adapter in `store/sql`, and an optional HTTP layer for query parameters and `Link` headers in `http`:

```go
import (
    "github.com/exalynt/turn/codec"
    "github.com/exalynt/turn/codec/plain"
    turnhttp "github.com/exalynt/turn/http"
    "github.com/exalynt/turn/paginator"
    "github.com/exalynt/turn/paginator/cursor"
    "github.com/exalynt/turn/paginator/offset"
    turnsql "github.com/exalynt/turn/store/sql"
    "github.com/exalynt/turn/store"
)
```

The MongoDB adapter is a separate module, so only services that use it depend on the MongoDB driver:

```sh
go get github.com/exalynt/turn/store/mongo
```

```go
import turnmongo "github.com/exalynt/turn/store/mongo"
```

turn's `http`, `store/sql`, and `store/mongo` packages share their names with `net/http`, `database/sql`, and the MongoDB driver's `mongo`, so the examples import them as `turnhttp`, `turnsql`, and `turnmongo`.

## How it works

turn supports two strategies:

- **Numbered pages** (`offset.Paginator`): page 1, 2, 3, … backed by `LIMIT`/`OFFSET`.
- **Cursors** (`cursor.Paginator`): keyset pagination that continues from the last item a client saw.

Both follow the same three steps:

1. **Prepare** a plan from a selector, which says which page the client wants. This validates the selector and works out what your query needs.
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

Create a paginator once per listing, for example when you build a repository, and reuse it. Paginators are safe for concurrent use. Everything specific to one selection lives in the plan.

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
func (r *UserRepository) ListByPage(ctx context.Context, selector offset.Selector) (paginator.Page[User, offset.Info], error) {
    plan, err := r.offset.Prepare(selector)
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

`offset.Selector{Number: 3, Size: 25}` prepares `Offset` 50 and `FetchLimit()` 26. A zero `Number` means page 1, and a zero `Size` means the policy's default. A page past the end of the data isn't an error; it comes back empty.

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

`Prepare` takes a scope alongside the selector. The scope names what a cursor is valid for: the resource, the filters, the caller's authorization scope, and the ordering. Your codec receives it when encoding and decoding, so a cursor from one listing can't be replayed against another.

The plan tells your query where to start and which way to read:

- **`cursor.Forward`** selects items strictly after `plan.Boundary`, in canonical order.
- **`cursor.Backward`** selects items strictly before `plan.Boundary`, in **reverse** canonical order. `Finish` flips them back.
- A nil `Boundary` (no cursor in the selector) starts at the beginning for `Forward` and at the end for `Backward`.

```go
func (r *UserRepository) ListByCursor(ctx context.Context, selector cursor.Selector) (paginator.Page[User, cursor.Info], error) {
    plan, err := r.cursor.Prepare(selector, "users:created_at_desc")
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

`page.Info` holds `StartCursor` and `EndCursor`, which are the positions of the first and last items in canonical order. Both are empty when the page has no items. It also echoes the selector: `Direction`, the resolved `Size`, and `Cursor`, the cursor the page was read from (empty for the first page).

```go
// Next page, reading forward.
if page.HasMore {
    next := cursor.Selector{Cursor: page.Info.EndCursor}
}

// Previous page, reading backward.
previous := cursor.Selector{Direction: cursor.Backward, Cursor: page.Info.StartCursor}
```

`HasMore` only covers the direction the page was read in. A forward page doesn't report whether earlier items exist, and a backward page doesn't report whether later ones do. A non-empty `page.Info.Cursor` means the page was read from a boundary, so items existed on its other side.

## Querying with adapters

The examples above build the query by hand. For cursors, that means writing the keyset predicate, flipping it and the sort for `Backward`, and numbering placeholders, which is easy to get subtly wrong. The optional `store` package and its adapters do that part for you. You still write the base query, run it, and scan the rows.

`store.Cursor` and `store.Offset` pair a paginator with the listing's sort order. Build them once, next to the paginators, for example when you build the repository:

```go
r.users, err = store.NewCursor(r.cursor,
    store.Desc("created_at", func(p UserPosition) any { return p.CreatedAt }),
    store.Desc("id", func(p UserPosition) any { return p.ID }),
)

r.usersByPage, err = store.NewOffset(r.offset,
    store.Sort{Field: "created_at", Desc: true},
    store.Sort{Field: "id", Desc: true},
)
```

Each key names a field and, for cursors, how to read its value from a position. List the keys in the same order as your position describes, ending with a unique one.

`List` runs the whole prepare, query, finish flow. It hands your function a `store.Query`, which says how to sort, where the keyset boundary lies, and how many items to fetch. An adapter turns that into your database's syntax.

### SQL

`turnsql.Render` returns a `turnsql.Clause`: the keyset predicate, `ORDER BY`, the limit clause, and their arguments. The third argument numbers the first placeholder, so the fragments can follow your own arguments. This is the cursor example from above:

```go
func (r *UserRepository) ListByCursor(ctx context.Context, selector cursor.Selector) (paginator.Page[User, cursor.Info], error) {
    return r.users.List(ctx, selector, "users:active:created_at_desc", func(ctx context.Context, q store.Query) ([]User, error) {
        c := turnsql.Render(turnsql.Postgres, q, 2)
        rows, err := r.db.QueryContext(ctx,
            "SELECT id, name, created_at FROM users WHERE status = $1"+c.And()+c.Tail(),
            append([]any{"active"}, c.Args...)...)
        if err != nil {
            return nil, err
        }
        return scanUsers(rows)
    })
}
```

`c.And()` appends the predicate to your `WHERE` clause; use `c.Filter()` instead for a query without one. `c.Tail()` adds `ORDER BY` and the limit. Reading forward from a cursor, the query becomes:

```sql
SELECT id, name, created_at FROM users WHERE status = $1
AND ((created_at, id) < ($2, $3)) ORDER BY created_at DESC, id DESC LIMIT $4
```

| Dialect | Placeholders | Keyset predicate |
| --- | --- | --- |
| `turnsql.Postgres` | `$1`, `$2`, … | A row comparison, `(a, b) < ($1, $2)`, when every key sorts the same direction |
| `turnsql.MySQL` | `?` | Always one key at a time, since MySQL doesn't reliably use an index for row comparisons |

With keys sorted in different directions, both compare one key at a time and bound the first key so an index still applies:

```sql
created_at <= $2 AND (created_at < $3 OR (created_at = $4 AND id > $5))
```

### MongoDB

`turnmongo.Render` returns the filter, sort, limit, and skip for a `Find`. `f.And` combines the keyset filter with yours:

```go
page, err := r.users.List(ctx, selector, "users:active:created_at_desc", func(ctx context.Context, q store.Query) ([]User, error) {
    f := turnmongo.Render(q)
    cur, err := r.coll.Find(ctx, f.And(bson.D{{Key: "status", Value: "active"}}), f.Options())
    if err != nil {
        return nil, err
    }
    var users []User
    if err := cur.All(ctx, &users); err != nil {
        return nil, err
    }
    return users, nil
})
```

Position values must encode to the same BSON type as the stored field. If documents hold a `bson.ObjectID`, keep a `bson.ObjectID` in the position, not its hex string.

### Writing your own adapter

For another SQL database, implement `turnsql.Dialect`. SQL Server, for example, uses named placeholders and `OFFSET … FETCH`:

```go
type sqlServer struct{}

func (sqlServer) Placeholder(n int) string { return "@p" + strconv.Itoa(n) }
func (sqlServer) RowValues() bool          { return false }
func (sqlServer) Limit(limit, offset string) string {
    if offset == "" {
        offset = "0"
    }
    return "OFFSET " + offset + " ROWS FETCH NEXT " + limit + " ROWS ONLY"
}
```

For any other store, render a `store.Query` directly. A query never carries a direction: for a backward read, `Sort` is already reversed, so your adapter always selects items strictly after `After` in `Sort` order, then sorts by `Sort` and fetches at most `Limit` items after skipping `Offset`. `q.Seek()` expands the boundary into conditions for stores that can't compare several values at once:

```go
// Sorted by created_at descending, then id ascending:
// [[created_at < x], [created_at = x, id > y]]
for _, conds := range q.Seek() {
    // an item is after the boundary if it matches every cond of any one group
}
```

### Things to know about adapters

- **Fields are emitted verbatim.** That lets you use qualified names and expressions, such as `u.created_at`, but a field must never come from request input. If clients choose the sort, map their choice through an allowlist of orders you built yourself.
- **Keyset values can't be null.** A comparison with `NULL` matches nothing, so pages would silently stop. Sort on non-null columns. A cursor whose boundary holds a null is rejected with `ErrInvalidCursor`.
- **Use the same order everywhere.** The keys, the position type, and the scope describe one ordering. Change them together, and change the scope so old cursors are rejected.

## Serving over HTTP

The core packages know nothing about HTTP, and turn never writes responses: the status, headers, and body are yours. The optional `github.com/exalynt/turn/http` package provides defaults for the two parts every listing endpoint repeats: reading a selector from the URL query, and linking to adjacent pages.

### Reading selectors

`turnhttp.OffsetQuery` and `turnhttp.CursorQuery` parse a selector from URL query parameters. Their zero values are ready to use:

| Type | Parameters |
| --- | --- |
| `turnhttp.OffsetQuery` | `?page=<n>`, `?size=<n>` |
| `turnhttp.CursorQuery` | `?after=<cursor>` reads forward, `?before=<cursor>` reads backward, `?size=<n>`. An empty `?before=` reads backward from the end. |

The defaults are the `turnhttp.Param` constants `ParamPage`, `ParamSize`, `ParamAfter`, and `ParamBefore`. To use other names, set them, for example `turnhttp.OffsetQuery{Page: "p", Size: "per_page"}`. Parameter names are part of your API, so changing them breaks URLs your clients already hold. If your API carries pagination some other way, such as in a request body, build the `offset.Selector` or `cursor.Selector` yourself.

`Parse` only parses: it reports a value that isn't an integer, or both `after` and `before`, with the same errors as `Prepare`, which still checks the size against the policy and decodes the cursor. Use `errors.Is` to tell a client's bad input from your own failures:

```go
var q turnhttp.OffsetQuery

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
    selector, err := q.Parse(r.URL.Query())
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    page, err := h.users.ListByPage(r.Context(), selector)
    switch {
    case isBadRequest(err):
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    case err != nil:
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Link", turnhttp.FormatLinks(turnhttp.OffsetLinks(page, q.URL(r.URL))...))
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

### Link headers

The [`Link` header](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Link) (RFC 8288) is the standard way to point clients at adjacent pages. `turnhttp.OffsetLinks` and `turnhttp.CursorLinks` work out which pages to link to from a finished page, and call a function you supply to get each one's URL. `turnhttp.FormatLinks` turns the result into a header value:

```http
Link: </users?page=1&size=25>; rel="first", </users?page=3&size=25>; rel="next"
```

`q.URL(r.URL)` is that function for the default parameters: it sets the pagination parameters on the current request URL and keeps the rest, such as filters, so links round-trip through `Parse`. For other URL shapes, pass your own function; it receives the selector for the linked page and returns its URL. Set the header before writing the body. Relative URLs like these are valid; clients resolve them against the request URL.

| Function | Links | Rules |
| --- | --- | --- |
| `turnhttp.OffsetLinks` | `first`, `prev`, `next` | `prev` above page 1, `next` when `HasMore`. No `last`, since turn doesn't count items. |
| `turnhttp.CursorLinks` | `first`, `prev`, `next`, `last` | `next` reads forward from `EndCursor` and `prev` backward from `StartCursor`, each when items are known to exist that way. `first` reads forward from the start and `last` backward from the end. |

Every selector your function receives carries the page's size, so following a link keeps it. Return nil to leave a link out, for example `last` if your API can't express reading backward from the end. Since the links builders return a `[]turnhttp.Link`, you can also put the links in a response body instead of a header.

### Errors

| Error | Cause | Typical response |
| --- | --- | --- |
| `ErrInvalidSize` | Size is negative, above `MaxSize`, or not an integer | 400 |
| `ErrInvalidPage` | Page number is negative or not an integer | 400 |
| `ErrOffsetTooLarge` | Page is deeper than `MaxOffset` allows, or the offset overflows | 400 |
| `ErrInvalidDirection` | Direction isn't `Forward` or `Backward`, or a query sets both `after` and `before` | 400 |
| `ErrInvalidCursor` | The codec couldn't decode the cursor for this scope, or a keyset value in it is null | 400 |
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
