module github.com/scovl/ollanta/ollantaweb

go 1.21

require (
	github.com/go-chi/chi/v5 v5.2.1
	github.com/golang-jwt/jwt/v4 v4.5.1
	github.com/jackc/pgx/v5 v5.7.2
	github.com/scovl/ollanta/application v0.0.0
	github.com/scovl/ollanta/domain v0.0.0
	github.com/scovl/ollanta/ollantacore v0.0.0
	github.com/scovl/ollanta/ollantaengine v0.0.0
	github.com/scovl/ollanta/ollantastore v0.0.0
	golang.org/x/crypto v0.31.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/text v0.21.0 // indirect
)

replace (
	github.com/scovl/ollanta/application => ../application
	github.com/scovl/ollanta/domain => ../domain
	github.com/scovl/ollanta/ollantacore => ../ollantacore
	github.com/scovl/ollanta/ollantaengine => ../ollantaengine
	github.com/scovl/ollanta/ollantastore => ../ollantastore
)
