# turn

[![Stability: Alpha](https://img.shields.io/badge/stability-alpha-f97316)](https://readme.exalynt.com/how-it-works/stability-levels)

A lightweight Go library for numbered-page and cursor pagination.

turn handles the parts of pagination every listing repeats: bounding the requested page size, turning "give me this page" into what your query needs, and shaping the result with what a client needs to fetch the next page. It never runs queries. Your code keeps the query, filters, authorization, execution, record mapping, and response format.

The core depends only on the standard library. Two optional layers sit on top, and you can use either, both, or neither:

- **`store`** renders the pagination parts of a query, such as the keyset `WHERE` and `ORDER BY`, for PostgreSQL, MySQL, MongoDB, or your own store.
- **`http`** reads page selections from URL query parameters and builds `Link` headers.

> [!WARNING]
> turn is in **Alpha**, one of Exalynt's [stability levels](https://readme.exalynt.com/how-it-works/stability-levels). It works, but it is still taking shape. Expect frequent breaking changes, sometimes without notice, and expect bugs. Build on it only to experiment, and expect its interfaces to change.

**Contents:** [Install](#install) · [Quick start](#quick-start) · [The core model](#the-core-model) · [Numbered pages](#numbered-pages) · [Cursors](#cursors) · [Errors](#errors) · [Things to get right](#things-to-get-right) · [Optional: query adapters](#optional-query-adapters) · [Optional: HTTP](#optional-http) · [Package layout](#package-layout)

## Install

```sh
go get github.com/exalynt/turn
```

turn needs Go 1.23 or newer.

## Quick start

This lists users from PostgreSQL in numbered pages, newest first, using `database/sql`:

```go
import "github.com/exalynt/turn/paginator/offset"

type User struct {
    ID        string
    Name      string
    CreatedAt time.Time
}

type UserRepository struct {
    db    *sql.DB
    pages *offset.Paginator[User]
}

func NewUserRepository(db *sql.DB) (*UserRepository, error) {
    // Build the paginator once and reuse it. The zero Options allow
    // 25 items per page by default and at most 100.
    pages, err := offset.New[User](offset.Options{})
    if err != nil {
        return nil, err
    }
    return &UserRepository{db: db, pages: pages}, nil
}

func (r *UserRepository) List(ctx context.Context, selector offset.Selector) (offset.Page[User], error) {
    // 1. Prepare: validate the selection and work out what the query needs.
    plan, err := r.pages.Prepare(selector)
    if err != nil {
        return offset.Page[User]{}, err
    }

    // 2. Query: your SQL, your filters, your row mapping.
    rows, err := r.db.QueryContext(ctx, `
        SELECT id, name, created_at FROM users
        ORDER BY created_at DESC, id DESC
        LIMIT $1 OFFSET $2`,
        plan.Limit(), plan.Offset)
    if err != nil {
        return offset.Page[User]{}, err
    }
    users, err := scanUsers(rows) // your own code
    if err != nil {
        return offset.Page[User]{}, err
    }

    // 3. Finish: shape the fetched rows into a page.
    return r.pages.Finish(plan, users)
}
```

Calling it:

```go
page, err := repo.List(ctx, offset.Selector{Number: 2})

page.Items   // up to 25 users
page.Number  // 2
page.Size    // 25
page.HasMore // true if page 3 has items
```

Cursor pagination follows the same three steps. See [Cursors](#cursors).

## The core model

### Four types

Every listing uses the same four pieces. The selector, plan, and page differ between the two strategies:

| Piece | What it is | Numbered pages | Cursors |
| --- | --- | --- | --- |
| **Paginator** | Built once per listing. Holds the size policy and, for cursors, how to encode positions. | `offset.Paginator[T]` | `cursor.Paginator[T, P]` |
| **Selector** | Which page the client asked for. The zero value means the first page at the default size. | `offset.Selector{Number, Size}` | `cursor.Selector{Direction, Size, Cursor}` |
| **Plan** | What your query needs to fetch that page. | `offset.Plan`: `Offset`, `Limit()` | `cursor.Plan[P]`: `Direction`, `Boundary`, `Limit()` |
| **Page** | The result: the items plus what a client needs to navigate. | `offset.Page[T]`: `Number`, `Size` | `cursor.Page[T]`: `Direction`, `Size`, `Cursor`, `StartCursor`, `EndCursor` |

`T` is your item type. `P` is a cursor *position*, the values that locate one item in the listing's order (see [Define a position](#define-a-position)).

### Prepare, query, finish

1. **Prepare** a plan from a selector. This validates the selector against the size policy and works out what the query needs.
2. **Query** your own store using the plan, fetching at most `plan.Limit()` items.
3. **Finish** the plan with the fetched items to get a page.

`Limit()` is the page size plus one. That extra lookahead item tells `Finish` whether more items exist, so neither strategy needs a `COUNT` query. `Finish` removes it before returning the page. Both page types hold the same two fields first, followed by what that strategy needs to navigate:

```go
Items   []T  // at most Size items, in canonical order; never nil
HasMore bool // whether the query found items beyond this page
```

Paginators are safe for concurrent use. Create one per listing, for example when you build a repository, and reuse it. Everything specific to one request lives in the plan.

### Page size policy

Both paginators take a `paginator.Policy`:

```go
paginator.Policy{DefaultSize: 20, MaxSize: 50}
```

A zero `Size` in a selector means `DefaultSize`. A size above `MaxSize` is rejected with `ErrInvalidSize`, not clamped. The zero `Policy` defaults to 25 and allows up to 100.

### Choosing a strategy

| | Numbered pages | Cursors |
| --- | --- | --- |
| Client can jump to page *N* | Yes | No, only next and previous |
| Stable while data changes | No: inserts and deletes shift pages | Yes, unless an item's sort values change |
| Cost of deep pages | Grows with depth, since the database reads every skipped row | Constant with a suitable index |
| What you provide | A sort order | A sort order, a position type, a codec, and a scope |

Use numbered pages for small or bounded listings, and when a UI needs page numbers. Use cursors for large, growing, or infinite-scroll listings and for APIs.

## Numbered pages

### Set up

```go
pages, err := offset.New[User](offset.Options{})
```

To set the sizes, or cap how deep clients can page, set `Policy` and `MaxOffset`:

```go
maxOffset := int64(10_000)

pages, err := offset.New[User](offset.Options{
    Policy:    paginator.Policy{DefaultSize: 20, MaxSize: 50},
    MaxOffset: &maxOffset,
})
```

A page deeper than `MaxOffset` is rejected with `ErrOffsetTooLarge`.

### Query

The plan's `Offset` and `Limit()` map directly onto `OFFSET` and `LIMIT`, as in the [quick start](#quick-start). `offset.Selector{Number: 3, Size: 25}` prepares `Offset` 50 and `Limit()` 26. A zero `Number` means page 1. A page past the end of the data isn't an error; it comes back empty.

### Navigate

The page holds its `Number` and `Size`. When `page.HasMore` is true, `Number + 1` is the next page. A `Number` above 1 means there's an earlier page to link to, though turn doesn't check whether it still has items. Keep `Size` the same between requests, since changing it moves which items each page number covers.

## Cursors

Cursor (keyset) pagination continues from the last item a client saw, rather than skipping a number of rows. Setting it up takes four decisions: a position type, a codec, a scope, and the queries for each direction.

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

A `codec.Codec` turns positions into the opaque `codec.Cursor` strings that clients hold, and back again. The default, `plain.Codec` from `github.com/exalynt/turn/codec/plain`, encodes the position as JSON behind a version byte and a digest of the scope, then as unpadded base64url, so cursors go in a URL without escaping. Its zero value is ready to use.

Your position type must round-trip through `encoding/json` exactly. `time.Time` does, at nanosecond precision.

`plain` cursors aren't signed: a client can read the position and forge another. To sign or encrypt cursors, or to use another format, implement `codec.Codec` yourself:

```go
type Codec[P any] interface {
    Encode(position P, scope string) (Cursor, error)
    Decode(cursor Cursor, scope string) (P, error)
}
```

Keep these in mind when writing your own codec:

- **Version the encoding.** Clients keep cursors across your deploys, so a cursor issued by the previous release should still decode.
- **Check the scope.** `Decode` should reject cursors issued for a different scope (see below).
- **Encoding isn't protection.** Base64 hides nothing, and anyone can forge a cursor. If that matters, sign cursors in your codec, for example with HMAC. Cursor checks never replace your own authorization.
- **Preserve values exactly.** A position that loses precision on a round trip, such as a truncated timestamp, can skip or repeat items.

### Create the paginator

```go
cursors, err := cursor.New(cursor.Options[User, UserPosition]{
    Codec: plain.Codec[UserPosition]{},
    Position: func(u User) UserPosition {
        return UserPosition{CreatedAt: u.CreatedAt, ID: u.ID}
    },
})
```

`Codec` and `Position` are required, and both must be safe for concurrent use. `Policy` works the same as for numbered pages.

### Name the scope

`Prepare` takes a scope alongside the selector. The scope is a string naming what a cursor is valid for: the resource, the filters, the caller's authorization scope, and the ordering. Your codec receives it when encoding and decoding, so a cursor from one listing can't be replayed against another. A listing of active users, newest first, might use `"users:status=active:created_at_desc"`.

### Query by cursor

The plan tells your query where to start and which way to read:

- **`cursor.Forward`** selects items strictly after `plan.Boundary`, in canonical order.
- **`cursor.Backward`** selects items strictly before `plan.Boundary`, in **reverse** canonical order. `Finish` flips them back.
- A nil `Boundary` (no cursor in the selector) starts at the beginning for `Forward` and at the end for `Backward`.

```go
func (r *UserRepository) ListByCursor(ctx context.Context, selector cursor.Selector) (cursor.Page[User], error) {
    plan, err := r.cursors.Prepare(selector, "users:created_at_desc")
    if err != nil {
        return cursor.Page[User]{}, err
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
    args = append(args, plan.Limit())

    rows, err := r.db.QueryContext(ctx, query, args...)
    if err != nil {
        return cursor.Page[User]{}, err
    }
    users, err := scanUsers(rows)
    if err != nil {
        return cursor.Page[User]{}, err
    }

    return r.cursors.Finish(plan, users)
}
```

If the listing has filters, apply the same filters to the query and include them in the scope.

Writing the keyset predicate and flipping it for `Backward` by hand is easy to get subtly wrong. The optional [query adapters](#optional-query-adapters) generate that part.

### Navigate with cursors

The page holds `StartCursor` and `EndCursor`, the cursors of the first and last items in canonical order. Both are empty when the page has no items. It also echoes the selector: `Direction`, the resolved `Size`, and `Cursor`, the cursor the page was read from (empty for the first page).

```go
// Next page, reading forward.
if page.HasMore {
    next := cursor.Selector{Cursor: page.EndCursor}
}

// Previous page, reading backward.
previous := cursor.Selector{Direction: cursor.Backward, Cursor: page.StartCursor}
```

`HasMore` only covers the direction the page was read in. A forward page doesn't report whether earlier items exist, and a backward page doesn't report whether later ones do. A non-empty `page.Cursor` means the page was read from a boundary, so items existed on its other side.

## Errors

Every error turn returns wraps one of these sentinels from the `paginator` package, so check them with `errors.Is`. They fall into two groups. Selector errors mean the page selection you were given is invalid, whether it came from an HTTP request, command-line flags, or anywhere else. Usage errors mean a bug in the calling code.

| Error | Group | Cause |
| --- | --- | --- |
| `ErrInvalidSize` | Selector | Size is negative or above `MaxSize` (or, from `http`, not an integer) |
| `ErrInvalidPage` | Selector | Page number is negative (or, from `http`, not an integer) |
| `ErrOffsetTooLarge` | Selector | Page is deeper than `MaxOffset` allows, or the offset overflows |
| `ErrInvalidDirection` | Selector | Direction isn't `Forward` or `Backward` (or, from `http`, both `after` and `before` are set) |
| `ErrInvalidCursor` | Selector | The codec couldn't decode the cursor for this scope, or (from `store`) a keyset value in it is null |
| `ErrInvalidOptions` | Usage | A paginator's or `store` wrapper's configuration is invalid; fix it at startup |
| `ErrInvalidPlan` | Usage | `Finish` got a plan this paginator didn't prepare, or one that was changed |
| `ErrInvalidBatch` | Usage | The query returned more than `Limit()` items |

`Finish` on a cursor paginator can also return an error from your codec's `Encode`.

`paginator.IsSelectorError` reports whether an error is in the selector group, so you can report it as bad input, such as an HTTP 400 or a CLI usage message, and treat everything else as your own failure:

```go
page, err := repo.List(ctx, selector)
switch {
case paginator.IsSelectorError(err):
    // bad input: tell whoever supplied the selector
case err != nil:
    // a bug or a failed query
}
```

## Things to get right

- **Use a deterministic order** that ends with a unique column, such as `id`. Without one, items with equal sort values can repeat or go missing between pages.
- **Return the whole query result to `Finish`.** Fewer than `Limit()` items tells turn the listing is exhausted. If the query fails, return its error rather than finishing a partial batch.
- **Don't change plans.** Pass the plan from `Prepare` to `Finish` as it is. `Finish` rejects plans it couldn't have produced.
- **Neither strategy is a snapshot.** Inserts and deletes between requests shift numbered pages, and updates to sort columns can move items across cursor boundaries. Transactions and snapshots are up to you.
- **Deep offsets are slow.** The database still reads every skipped row. Set `MaxOffset`, or use cursors for large or unbounded listings.

`Finish` never reorders or modifies the slice you pass in, and appending to `page.Items` never overwrites it.

## Optional: query adapters

Everything above works with the core packages alone. The `store` package and its adapters are an optional convenience: they generate the pagination parts of a query, such as the keyset predicate, its flip for `Backward`, the sort, the limit, and placeholder numbering. You still write the base query, run it, and map the results. Nothing in the core or in `http` depends on `store`.

### Describe the sort order

`store.Cursor` and `store.Offset` pair a paginator with the listing's sort order. Build them once, next to the paginators:

```go
r.usersByCursor, err = store.NewCursor(cursors,
    store.Desc("created_at", func(p UserPosition) any { return p.CreatedAt }),
    store.Desc("id", func(p UserPosition) any { return p.ID }),
)

r.usersByPage, err = store.NewOffset(pages,
    store.Sort{Field: "created_at", Desc: true},
    store.Sort{Field: "id", Desc: true},
)
```

Each key names a field and, for cursors, how to read its value from a position. List the keys in the same order as your position describes, ending with a unique one.

`List` runs the whole prepare, query, finish flow. It hands your function a `store.Query`, which says how to sort, where the keyset boundary lies, and how many items to fetch. An adapter turns that into your database's syntax.

### SQL

```go
import turnsql "github.com/exalynt/turn/store/sql"
```

`turnsql.Render` returns a `turnsql.Clause`: the keyset predicate, `ORDER BY`, the limit clause, and their arguments. The third argument numbers the first placeholder, so the fragments can follow your own arguments. This is the cursor example from above, with a filter added:

```go
func (r *UserRepository) ListByCursor(ctx context.Context, selector cursor.Selector) (cursor.Page[User], error) {
    return r.usersByCursor.List(ctx, selector, "users:status=active:created_at_desc", func(ctx context.Context, q store.Query) ([]User, error) {
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

The MongoDB adapter is a separate module, so only services that use it depend on the MongoDB driver:

```sh
go get github.com/exalynt/turn/store/mongo
```

```go
import turnmongo "github.com/exalynt/turn/store/mongo"
```

`turnmongo.Render` returns the filter, sort, limit, and skip for a `Find`. `f.And` combines the keyset filter with yours:

```go
page, err := r.usersByCursor.List(ctx, selector, "users:status=active:created_at_desc", func(ctx context.Context, q store.Query) ([]User, error) {
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

For any other store, render a `store.Query` directly. A query never carries a direction: for a backward read, `Sort` is already reversed. Your adapter always selects items strictly after `After` in `Sort` order, sorts by `Sort`, skips `Offset` items, and fetches at most `Limit`. `q.Seek()` expands the boundary into conditions for stores that can't compare several values at once:

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

## Optional: HTTP

```go
import turnhttp "github.com/exalynt/turn/http"
```

The core knows nothing about HTTP, and turn never writes responses: the status, headers, and body are yours. The optional `http` package covers the two parts every listing endpoint repeats: reading a selector from the URL query, and linking to adjacent pages. It works with any repository code, whether or not it uses `store`.

### Reading selectors

`turnhttp.OffsetQuery` and `turnhttp.CursorQuery` parse a selector from URL query parameters. Their zero values are ready to use:

| Type | Parameters |
| --- | --- |
| `turnhttp.OffsetQuery` | `?page=<n>`, `?size=<n>` |
| `turnhttp.CursorQuery` | `?after=<cursor>` reads forward, `?before=<cursor>` reads backward, `?size=<n>`. An empty `?before=` reads backward from the end. |

To use other names, set the fields, for example `turnhttp.OffsetQuery{Page: "p", Size: "per_page"}`. The defaults are the constants `ParamPage`, `ParamSize`, `ParamAfter`, and `ParamBefore`. Parameter names are part of your API, so changing them breaks URLs your clients already hold. If your API carries pagination some other way, such as in a request body, build the `offset.Selector` or `cursor.Selector` yourself.

`Parse` only parses. It reports a value that isn't an integer, or both `after` and `before`, using the same [errors](#errors) as `Prepare`, which still checks the size against the policy and decodes the cursor:

```go
var q turnhttp.OffsetQuery

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
    selector, err := q.Parse(r.URL.Query())
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    page, err := h.users.List(r.Context(), selector)
    switch {
    case paginator.IsSelectorError(err):
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    case err != nil:
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Link", turnhttp.FormatLinks(turnhttp.OffsetLinks(page, q.URL(r.URL))...))
    json.NewEncoder(w).Encode(map[string]any{
        "items":    page.Items,
        "page":     page.Number,
        "size":     page.Size,
        "has_more": page.HasMore,
    })
}
```

### Link headers

The [`Link` header](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Link) (RFC 8288) is the standard way to point clients at adjacent pages. `turnhttp.OffsetLinks` and `turnhttp.CursorLinks` work out which pages to link to from a finished page, and call a function you supply to get each one's URL. `turnhttp.FormatLinks` turns the result into a header value:

```http
Link: </users?page=1&size=25>; rel="first", </users?page=3&size=25>; rel="next"
```

`q.URL(r.URL)` is that function for the default parameters. It sets the pagination parameters on the current request URL and keeps the rest, such as filters, so links round-trip through `Parse`. For other URL shapes, pass your own function; it receives the selector for the linked page and returns its URL. Set the header before writing the body. Relative URLs like these are valid; clients resolve them against the request URL.

| Function | Links | Rules |
| --- | --- | --- |
| `turnhttp.OffsetLinks` | `first`, `prev`, `next` | `prev` above page 1, `next` when `HasMore`. No `last`, since turn doesn't count items. |
| `turnhttp.CursorLinks` | `first`, `prev`, `next`, `last` | `next` reads forward from `EndCursor` and `prev` backward from `StartCursor`, each when items are known to exist that way. `first` reads forward from the start and `last` backward from the end. |

Every selector your function receives carries the page's size, so following a link keeps it. Return nil to leave a link out, for example `last` if your API can't express reading backward from the end. The link builders return a `[]turnhttp.Link`, so you can put the links in a response body instead of a header.

## Package layout

| Package | Import as | Holds | Needed? |
| --- | --- | --- | --- |
| `github.com/exalynt/turn/paginator` | `paginator` | `Policy`, `Window`, and the errors | Always |
| `github.com/exalynt/turn/paginator/offset` | `offset` | The numbered-page paginator and its `Page` | For numbered pages |
| `github.com/exalynt/turn/paginator/cursor` | `cursor` | The cursor paginator and its `Page` | For cursors |
| `github.com/exalynt/turn/codec` | `codec` | The `Codec` interface and `Cursor` type | For cursors |
| `github.com/exalynt/turn/codec/plain` | `plain` | The default, unsigned codec | Optional |
| `github.com/exalynt/turn/store` | `store` | The store-independent query description | Optional |
| `github.com/exalynt/turn/store/sql` | `turnsql` | The SQL adapter | Optional |
| `github.com/exalynt/turn/store/mongo` | `turnmongo` | The MongoDB adapter, in its own module | Optional |
| `github.com/exalynt/turn/http` | `turnhttp` | URL query parsing and `Link` headers | Optional |

`http`, `store/sql`, and `store/mongo` share their names with `net/http`, `database/sql`, and the MongoDB driver's `mongo`, so the examples import them as `turnhttp`, `turnsql`, and `turnmongo`. Every package except `store/mongo` is in the core module and depends only on the standard library.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
