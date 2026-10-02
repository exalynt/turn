// Package turn provides numbered-page and cursor pagination primitives.
//
// turn handles the parts of pagination every listing repeats: bounding the
// requested page size, turning a page selection into what a query needs, and
// shaping the result. turn never runs queries; the consumer owns the base
// query, filtering, authorization, execution, record mapping, and response
// format.
//
// This package holds no code.
//
// # Core
//
// Every listing uses the core packages, which depend only on the standard
// library:
//
//   - [github.com/exalynt/turn/paginator]: what every paginator shares, the
//     size policy and the errors.
//   - [github.com/exalynt/turn/paginator/offset]: numbered pages.
//   - [github.com/exalynt/turn/paginator/cursor]: keyset pagination.
//   - [github.com/exalynt/turn/codec]: how cursor positions become opaque
//     cursors, with a default implementation in
//     [github.com/exalynt/turn/codec/plain].
//
// Each paginator follows the same three steps: plan, fetch, page. Plan turns a
// selector into a plan, the consumer fetches items with its own query using the
// plan, and Page builds a page from the plan and the fetched items.
//
// # Optional layers
//
// Two layers sit on top of the core. Neither is required, and neither depends
// on the other:
//
//   - [github.com/exalynt/turn/store]: describes a page fetch independently of
//     any database, so an adapter can render the keyset predicate, sort, and
//     limit.
//   - [github.com/exalynt/turn/store/sql]: the store adapter for PostgreSQL,
//     MySQL, or a consumer's own SQL dialect.
//   - github.com/exalynt/turn/store/mongo: the store adapter for MongoDB, in
//     its own module so that only its importers depend on the MongoDB driver.
//   - [github.com/exalynt/turn/http]: reads selectors from URL query
//     parameters and builds RFC 8288 Link headers.
//
// turn is in Alpha: it is still taking shape, so expect frequent breaking
// changes. See https://readme.exalynt.com/how-it-works/stability-levels.
package turn
