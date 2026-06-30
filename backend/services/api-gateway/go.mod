module github.com/bidwriter/services/api-gateway

go 1.23

require (
	github.com/bidwriter/shared v0.0.0-00010101000000-000000000000
	github.com/go-chi/chi/v5 v5.1.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.7.1
	golang.org/x/crypto v0.27.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/stretchr/testify v1.8.4 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/text v0.18.0 // indirect
)

replace github.com/bidwriter/shared => ../../shared
