package blend

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nekwebdev/confb/internal/config"
	"gopkg.in/yaml.v3"
)

func TestYAML_Deep_UniqueAppend(t *testing.T) {
	td := t.TempDir()
	base := filepath.Join(td, "base.yaml")
	over := filepath.Join(td, "overlay.yaml")

	// base
	writeFileT(t, base, `
services:
  web:
    image: app:v1
    env:
      DEBUG: false
      THEME: light
  db:
    engine: postgres
    ports: [5432]
`)

	// overlay
	writeFileT(t, over, `
services:
  web:
    env:
      DEBUG: true
    replicas: 3
  db:
    ports: [5433, 5432]
`)

	rules := &config.MergeRules{Maps: "deep", Arrays: "unique_append"}
	out, err := BlendStructured("yaml", rules, []string{base, over})
	if err != nil {
		t.Fatalf("BlendStructured(yaml) error: %v", err)
	}

	var got any
	if err := yaml.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal result: %v\nout:\n%s", err, out)
	}

	root := got.(map[string]any)
	svc := root["services"].(map[string]any)

	web := svc["web"].(map[string]any)
	if web["image"] != "app:v1" {
		t.Fatalf("web.image = %v, want app:v1", web["image"])
	}
	env := web["env"].(map[string]any)
	if env["DEBUG"] != true {
		t.Fatalf("web.env.DEBUG = %v, want true", env["DEBUG"])
	}
	if env["THEME"] != "light" {
		t.Fatalf("web.env.THEME = %v, want light", env["THEME"])
	}
	if web["replicas"] != 3 {
		t.Fatalf("web.replicas = %v, want 3", web["replicas"])
	}

	db := svc["db"].(map[string]any)
	ports, ok := toInt64Slice(db["ports"])
	if !ok {
		t.Fatalf("db.ports not numeric slice: %#v", db["ports"])
	}
	if !reflect.DeepEqual(ports, []int64{5432, 5433}) {
		t.Fatalf("db.ports = %v, want [5432 5433]", ports)
	}
	if db["engine"] != "postgres" {
		t.Fatalf("db.engine = %v, want postgres", db["engine"])
	}
}

func TestYAML_MapsReplace_ArraysReplace(t *testing.T) {
	td := t.TempDir()
	base := filepath.Join(td, "base.yaml")
	over := filepath.Join(td, "overlay.yaml")

	writeFileT(t, base, `
svc:
  arr: [1, 2, 3]
  nest:
    k: base
`)
	writeFileT(t, over, `
svc:
  arr: [9]
  nest:
    k: over
    x: 42
`)

	rules := &config.MergeRules{Maps: "replace", Arrays: "replace"}
	out, err := BlendStructured("yaml", rules, []string{base, over})
	if err != nil {
		t.Fatalf("BlendStructured(yaml) error: %v", err)
	}

	var got map[string]any
	if err := yaml.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal result: %v\nout:\n%s", err, out)
	}

	svc := got["svc"].(map[string]any)

	arr, ok := toInt64Slice(svc["arr"])
	if !ok || !reflect.DeepEqual(arr, []int64{9}) {
		t.Fatalf("svc.arr = %v, want [9]", svc["arr"])
	}

  nest := svc["nest"].(map[string]any)
	if nest["k"] != "over" {
		t.Fatalf("svc.nest.k = %v, want over", nest["k"])
	}
	// YAML numbers may decode to int/float; tolerate both.
	switch nest["x"].(type) {
	case int, int64, float64:
		// ok
	default:
		t.Fatalf("svc.nest.x type = %T (val=%v), want numeric", nest["x"], nest["x"])
	}
}
