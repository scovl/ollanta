module github.com/scovl/ollanta/ollantaweb

go 1.23.0

require (
	github.com/go-chi/chi/v5 v5.2.1
	github.com/golang-jwt/jwt/v4 v4.5.1
	github.com/jackc/pgx/v5 v5.7.2
	github.com/scovl/ollanta/adapter v0.0.0
	github.com/scovl/ollanta/application v0.0.0
	github.com/scovl/ollanta/domain v0.0.0
	github.com/scovl/ollanta/ollantacore v0.0.0
	github.com/scovl/ollanta/ollantaengine v0.0.0
	github.com/scovl/ollanta/ollantastore v0.0.0
	golang.org/x/crypto v0.41.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.0-20190314233015-f79a8a8ca69d // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.20.0 // indirect
	github.com/go-openapi/spec v0.20.6 // indirect
	github.com/go-openapi/swag v0.19.15 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/mailru/easyjson v0.7.6 // indirect
	github.com/russross/blackfriday/v2 v2.0.1 // indirect
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	github.com/swaggo/files v0.0.0-20220610200504-28940afbdbfe // indirect
	github.com/swaggo/http-swagger v1.3.4 // indirect
	github.com/swaggo/swag v1.16.6 // indirect
	github.com/urfave/cli/v2 v2.3.0 // indirect
	golang.org/x/mod v0.26.0 // indirect
	golang.org/x/net v0.43.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	golang.org/x/tools v0.35.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace (
	github.com/scovl/ollanta/adapter => ../adapter
	github.com/scovl/ollanta/application => ../application
	github.com/scovl/ollanta/domain => ../domain
	github.com/scovl/ollanta/ollantacore => ../ollantacore
	github.com/scovl/ollanta/ollantaengine => ../ollantaengine
	github.com/scovl/ollanta/ollantastore => ../ollantastore
)
