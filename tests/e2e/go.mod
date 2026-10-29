module github.com/exalynt/turn/tests/e2e

go 1.23.0

require (
	github.com/exalynt/turn v0.0.0
	github.com/exalynt/turn/store/mongo v0.0.0
	github.com/go-sql-driver/mysql v1.9.3
	github.com/jackc/pgx/v5 v5.7.6
	go.mongodb.org/mongo-driver/v2 v2.8.2
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/klauspost/compress v1.17.6 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.2.0 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/text v0.24.0 // indirect
)

// Test turn as it is in this repository, not a released version.
replace (
	github.com/exalynt/turn => ../../
	github.com/exalynt/turn/store/mongo => ../../store/mongo
)
