package ollantaparser_test

import (
	"testing"

	"github.com/scovl/ollanta/ollantaparser"
	"github.com/scovl/ollanta/ollantaparser/languages"
)

func TestRegistry_Register_ForExtension(t *testing.T) {
	r := ollantaparser.NewRegistry()
	r.Register(languages.JavaScript)

	lang, ok := r.ForExtension(".js")
	if !ok {
		t.Fatal("expected JavaScript registered for .js")
	}
	if lang.Name() != "javascript" {
		t.Errorf("Name: got %q", lang.Name())
	}
}

func TestRegistry_ForExtension_Mjs(t *testing.T) {
	r := ollantaparser.NewRegistry()
	r.Register(languages.JavaScript)

	lang, ok := r.ForExtension(".mjs")
	if !ok {
		t.Fatal("expected JavaScript registered for .mjs")
	}
	if lang.Name() != "javascript" {
		t.Errorf("Name: got %q", lang.Name())
	}
}

func TestRegistry_ForName(t *testing.T) {
	r := ollantaparser.NewRegistry()
	r.Register(languages.Python)

	lang, ok := r.ForName("python")
	if !ok {
		t.Fatal("expected Python registered by name")
	}
	if lang.Name() != "python" {
		t.Errorf("Name: got %q", lang.Name())
	}
}

func TestRegistry_ForExtension_Unknown(t *testing.T) {
	r := ollantaparser.NewRegistry()
	_, ok := r.ForExtension(".unknown_xyz")
	if ok {
		t.Error("expected not found for unknown extension")
	}
}

func TestDefaultRegistry_ContainsAllLanguages(t *testing.T) {
	r := languages.DefaultRegistry()

	cases := []struct {
		ext  string
		lang string
	}{
		{".js", "javascript"},
		{".mjs", "javascript"},
		{".ts", "typescript"},
		{".tsx", "typescript"},
		{".py", "python"},
		{".rs", "rust"},
	}

	for _, tc := range cases {
		l, ok := r.ForExtension(tc.ext)
		if !ok {
			t.Errorf("DefaultRegistry: missing extension %q", tc.ext)
			continue
		}
		if l.Name() != tc.lang {
			t.Errorf("ForExtension(%q): got %q, want %q", tc.ext, l.Name(), tc.lang)
		}
	}
}

func TestRegistry_ForFile(t *testing.T) {
	r := languages.DefaultRegistry()
	lang, ok := r.ForFile("src/app.js")
	if !ok {
		t.Fatal("ForFile: expected JS for src/app.js")
	}
	if lang.Name() != "javascript" {
		t.Errorf("Name: got %q", lang.Name())
	}
}
