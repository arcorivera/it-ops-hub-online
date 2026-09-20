module itopshub/backend

go 1.22.2

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/go-chi/cors v1.2.2
	github.com/google/uuid v1.6.0
	github.com/mattn/go-sqlite3 v1.14.49
	golang.org/x/crypto v0.31.0
)

replace golang.org/x/crypto => github.com/golang/crypto v0.31.0
