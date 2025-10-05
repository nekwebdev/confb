package blend

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nekwebdev/confb/internal/config"
	"github.com/pelletier/go-toml/v2"
)

func TestTOML_Deep_UniqueAppend(t *testing.T) {
	td := t.TempDir()
	base := filepath.Join(td, "base.toml")
	over := filepath.Join(td, "overlay.toml")

	writeFileT(t, base, `
[service]
name = "api"
ports = [8080]

[service.env]
DEBUG = false
`)
	writeFileT(t, over, `
[service]
ports = [9090, 8080]

[service.env]
DEBUG = true
NEW = "x"
`)

	rules := &config.MergeRules{Maps: "deep", Arrays: "unique_append"}
	out, err := BlendStructured("toml", rules, []string{base, over})
	if err != nil {
		t.Fatalf("BlendStructured(toml) error: %v", err)
	}

	var got any
	if err := toml.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal result: %v\nout:\n%s", err, out)
	}
	m := got.(map[string]any)
	svc := m["service"].(map[string]any)

	if svc["name"] != "api" {
		t.Fatalf("service.name = %v, want api", svc["name"])
	}

	ports, ok := toInt64Slice(svc["ports"])
	if !ok {
		t.Fatalf("service.ports not int slice: %#v", svc["ports"])
	}
	if !reflect.DeepEqual(ports, []int64{8080, 9090}) {
		t.Fatalf("service.ports = %v, want [8080 9090]", ports)
	}

	env := svc["env"].(map[string]any)
	if env["DEBUG"] != true {
		t.Fatalf("service.env.DEBUG = %v, want true", env["DEBUG"])
	}
	if env["NEW"] != "x" {
		t.Fatalf("service.env.NEW = %v, want x", env["NEW"])
	}
}

func TestTOML_MapsReplace_ArraysReplace(t *testing.T) {
	td := t.TempDir()
	base := filepath.Join(td, "base.toml")
	over := filepath.Join(td, "overlay.toml")

	writeFileT(t, base, `
[svc]
arr = [1,2,3]
[svc.nest]
k = "base"
`)
	writeFileT(t, over, `
[svc]
arr = [9]
[svc.nest]
k = "over"
x = 42
`)

	rules := &config.MergeRules{Maps: "replace", Arrays: "replace"}
	out, err := BlendStructured("toml", rules, []string{base, over})
	if err != nil {
		t.Fatalf("BlendStructured(toml) error: %v", err)
	}

	var got map[string]any
	if err := toml.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal result: %v\nout:\n%s", err, out)
	}
	svc := got["svc"].(map[string]any)

	arr, ok := toInt64Slice(svc["arr"])
	if !ok || !reflect.DeepEqual(arr, []int64{9}) {
		t.Fatalf("svc.arr = %v, want [9]", svc["arr"])
	}
	nest := svc["nest"].(map[string]any)
	if nest["k"] != "over" || nest["x"] != int64(42) {
		t.Fatalf("svc.nest = %#v, want {k:over x:42}", nest)
	}
}
