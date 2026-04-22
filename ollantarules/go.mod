module github.com/scovl/ollanta/ollantarules

go 1.21

replace (
	github.com/scovl/ollanta/ollantacore => ../ollantacore
	github.com/scovl/ollanta/ollantaparser => ../ollantaparser
)

require (
	github.com/scovl/ollanta/ollantacore v0.0.0-00010101000000-000000000000
	github.com/scovl/ollanta/ollantaparser v0.0.0-00010101000000-000000000000
)

require github.com/smacker/go-tree-sitter v0.0.0-20240827094217-dd81d9e9be82 // indirect
