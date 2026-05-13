package parser

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/scovl/ollanta/domain/port"
	"github.com/scovl/ollanta/ollantaparser"
)

type Parser struct {
	registry *LanguageRegistry
}

func NewParser(registry *LanguageRegistry) *Parser {
	return &Parser{registry: registry}
}

var _ port.IParser = (*Parser)(nil)

func (p *Parser) ParseFile(path, language string) (any, error) {
	lang, ok := p.registry.ForFile(path)
	if !ok && language != "" {
		lang, ok = p.registry.ForName(language)
	}
	if !ok {
		return nil, fmt.Errorf("parser: no grammar registered for %s", filepath.Ext(path))
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("parser: reading %s: %w", path, err)
	}
	return ollantaparser.Parse(path, src, lang)
}

func (p *Parser) ParseSource(path string, src []byte, language string) (any, error) {
	lang, ok := p.registry.ForName(language)
	if !ok {
		lang, ok = p.registry.ForFile(path)
	}
	if !ok {
		return nil, fmt.Errorf("parser: no grammar registered for language %q", language)
	}
	return ollantaparser.Parse(path, src, lang)
}
