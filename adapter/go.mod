module github.com/scovl/ollanta/adapter

go 1.21

require (
	github.com/scovl/ollanta/domain v0.0.0
	github.com/scovl/ollanta/ollantacore v0.0.0
	github.com/scovl/ollanta/ollantaparser v0.0.0
	github.com/scovl/ollanta/ollantarules v0.0.0
	github.com/smacker/go-tree-sitter v0.0.0-20240827094217-dd81d9e9be82
)

replace (
	github.com/scovl/ollanta/domain => ../domain
	github.com/scovl/ollanta/ollantacore => ../ollantacore
	github.com/scovl/ollanta/ollantaparser => ../ollantaparser
	github.com/scovl/ollanta/ollantarules => ../ollantarules
)
