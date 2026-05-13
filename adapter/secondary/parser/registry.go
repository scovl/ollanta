package parser

import (
	"path/filepath"
	"sync"

	"github.com/scovl/ollanta/ollantaparser"
)

type LanguageRegistry struct {
	mu     sync.RWMutex
	byExt  map[string]ollantaparser.Language
	byName map[string]ollantaparser.Language
}

func NewRegistry() *LanguageRegistry {
	return &LanguageRegistry{
		byExt:  make(map[string]ollantaparser.Language),
		byName: make(map[string]ollantaparser.Language),
	}
}

func (r *LanguageRegistry) Register(lang ollantaparser.Language) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byName[lang.Name()] = lang
	for _, ext := range lang.Extensions() {
		r.byExt[ext] = lang
	}
}

func (r *LanguageRegistry) ForExtension(ext string) (ollantaparser.Language, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	l, ok := r.byExt[ext]
	return l, ok
}

func (r *LanguageRegistry) ForName(name string) (ollantaparser.Language, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	l, ok := r.byName[name]
	return l, ok
}

func (r *LanguageRegistry) ForFile(path string) (ollantaparser.Language, bool) {
	return r.ForExtension(filepath.Ext(path))
}
