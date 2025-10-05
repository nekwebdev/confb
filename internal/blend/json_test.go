package blend

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nekwebdev/confb/internal/config"
)

func TestJSON_Deep_UniqueAppend(t *testing.T) {
	td := t.TempDir()
	base := filepath.Join(td, "base.json")
	over := filepath.Join(td, "overlay.json")

	writeFileT(t, base, `{
  "services": {
    "web": {
      "image": "app:v1",
      "env": { "DEBUG": false, "THEME": "light" }
    },
    "db": {
      "engine": "postgres",
      "ports": [5432]
    }
  }
}`)
	writeFileT(t, over, `{
  "services": {
    "web": {
      "env": { "DEBUG": true },
      "replicas": 3
    },
    "db": {
      "ports": [5433, 5432]
    }
  }
}`)

	rules := &config.MergeRules{Maps: "deep", Arrays: "unique_append"}
	out, err := BlendStructured("json", rules, []string{base, over})
	if err != nil {
		t.Fatalf("BlendStructured(json) error: %v", err)
	}

	var got any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
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
	// JSON numbers decode as float64
	if web["replicas"] != float64(3) {
		t.Fatalf("web.replicas = %v (%T), want 3 (float64)", web["replicas"], web["replicas"])
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

func TestJSON_MapsReplace_ArraysReplace(t *testing.T) {
	td := t.TempDir()
	base := filepath.Join(td, "base.json")
	over := filepath.Join(td, "overlay.json")

	writeFileT(t, base, `{
  "svc": {
    "arr": [1, 2, 3],
    "nest": { "k": "base" }
  }
}`)
	writeFileT(t, over, `{
  "svc": {
    "arr": [9],
    "nest": { "k": "over", "x": 42 }
  }
}`)

	rules := &config.MergeRules{Maps: "replace", Arrays: "replace"}
	out, err := BlendStructured("json", rules, []string{base, over})
	if err != nil {
		t.Fatalf("BlendStructured(json) error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal result: %v\nout:\n%s", err, out)
	}

	svc := got["svc"].(map[string]any)

	// arrays=replace -> [9]
	arr, ok := toInt64Slice(svc["arr"])
	if !ok || !reflect.DeepEqual(arr, []int64{9}) {
		t.Fatalf("svc.arr = %v, want [9]", svc["arr"])
	}

	// maps=replace -> overlay's nest entirely
	nest := svc["nest"].(map[string]any)
	if nest["k"] != "over" {
		t.Fatalf("svc.nest.k = %v, want over", nest["k"])
	}
	// JSON numbers are float64
	if nest["x"] != float64(42) {
		t.Fatalf("svc.nest.x = %v (%T), want 42 (float64)", nest["x"], nest["x"])
	}
}
