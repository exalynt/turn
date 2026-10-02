// Package turn provides page-based and cursor-based pagination primitives.
//
// turn handles the parts of pagination every listing repeats: bounding the
// requested page size, turning a request into what a query needs, and shaping
// the result. Its optional adapters render the pagination clauses of a query,
// but turn never runs queries; the consumer owns the base query, filtering,
// authorization, execution, and record mapping.
//
// This package holds no code. The packages are:
//
//   - [github.com/exalynt/turn/paginator]: what every paginator shares, the
//     size policy, the result page, and the errors.
//   - [github.com/exalynt/turn/paginator/offset]: numbered pages.
//   - [github.com/exalynt/turn/paginator/cursor]: keyset pagination.
//   - [github.com/exalynt/turn/codec]: how cursor positions become opaque
//     cursors, with a default implementation in
//     [github.com/exalynt/turn/codec/plain].
//   - [github.com/exalynt/turn/store]: the optional query layer, describing a
//     page fetch independently of any database, for adapters to render.
//   - [github.com/exalynt/turn/store/sql]: the adapter for SQL databases, rendering
//     keyset predicates, ORDER BY, and limits for PostgreSQL, MySQL, or a
//     consumer's own dialect.
//   - github.com/exalynt/turn/store/mongo: the adapter for MongoDB, in its own
//     module so that only its importers depend on the MongoDB driver.
//   - [github.com/exalynt/turn/http]: the optional HTTP layer, reading
//     selectors from URL query parameters and building RFC 8288 Link headers.
//
// turn is in Alpha: it is still taking shape, so expect frequent breaking
// changes. See https://readme.exalynt.com/how-it-works/stability-levels.
package turn
