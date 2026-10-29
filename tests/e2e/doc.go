// Package e2e tests turn the way a consumer uses it: it imports turn's
// exported packages only, and each test walks one common flow's happy path
// against a real database from compose.yml.
//
// The databases are reached through TURN_POSTGRES_URL, TURN_MYSQL_DSN, and
// TURN_MONGO_URI, which default to the host ports compose.override.example.yml
// publishes. Each test creates its own table or collection and drops it at the
// end, so the suite can run repeatedly and alongside other suites.
package e2e
