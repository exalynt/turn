# turn

[![Stability: Alpha](https://img.shields.io/badge/stability-alpha-f97316)](https://readme.exalynt.com/how-it-works/stability-levels)

A lightweight Go library for cursor and page-based pagination.

turn will handle the parts of pagination every service repeats: parsing and bounding the request, turning it into something a query can use, and shaping the response. It won't touch your database, and it will depend only on the standard library.

## Stability

turn is in **Alpha**, one of Exalynt's [stability levels](https://readme.exalynt.com/how-it-works/stability-levels). It works, but it is still taking shape. Expect frequent breaking changes, sometimes without notice, and expect bugs. Build on it only to experiment, and expect its interfaces to change.

The API hasn't been written yet. This repository currently contains only the project scaffolding.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
