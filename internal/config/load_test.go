package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekwebdev/confb/internal/config"
)

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestLoad_ValidConfig_AppliesDefaultsAndBaseDir(t *testing.T) {
	tmp := t.TempDir()

	// fake HOME so ~ expands deterministically
	fakeHome := filepath.Join(tmp, "home")
	if err := os.MkdirAll(fakeHome, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", fakeHome)
	defer os.Setenv("HOME", origHome)

	conf := `
version: 1
targets:
  - name: ok
    output: ~/out/file.conf
    # omit format/dedupe/newline/encoding -> defaults should kick in
    sources:
      - path: a/*.conf
`
	confPath := filepath.Join(tmp, "confb.yaml")
	mustWrite(t, confPath, conf)

	cfg, err := config.Load(confPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// baseDir should be the directory of confb.yaml
	base, err := cfg.BaseDir()
	if err != nil {
		t.Fatalf("BaseDir: %v", err)
	}
	if base != tmp {
		t.Fatalf("BaseDir mismatch: got %q want %q", base, tmp)
	}

	if len(cfg.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(cfg.Targets))
	}
	t0 := cfg.Targets[0]

	// defaults applied
	if t0.Format != "auto" {
		t.Fatalf("default format: got %q want %q", t0.Format, "auto")
	}
	if t0.Dedupe != "by_path" {
		t.Fatalf("default dedupe: got %q want %q", t0.Dedupe, "by_path")
	}
	if t0.Newline != "\n" {
		t.Fatalf("default newline: got %q want %q", t0.Newline, "\\n")
	}
	if strings.ToLower(t0.Encoding) != "utf8" {
		t.Fatalf("default encoding: got %q want utf8", t0.Encoding)
	}

	// ~ expanded
	wantPrefix := filepath.Join(fakeHome, "out")
	if !strings.HasPrefix(t0.Output, wantPrefix) {
		t.Fatalf("output tilde expansion: got %q want prefix %q", t0.Output, wantPrefix)
	}

	// source default sort applied
	if got := t0.Sources[0].Sort; got != "lex" {
		t.Fatalf("source default sort: got %q want %q", got, "lex")
	}
}

func TestLoad_InvalidConfig_AggregatesErrors(t *testing.T) {
	tmp := t.TempDir()

	bad := `
version: 2          # invalid version
targets:
  - name: ""        # empty name
    format: nope    # invalid enum
    output: ""      # missing output
    dedupe: wat     # invalid enum
    newline: "\r\n" # not allowed in MVP
    encoding: latin1
    sources: []
`
	confPath := filepath.Join(tmp, "confb.yaml")
	mustWrite(t, confPath, bad)

	_, err := config.Load(confPath)
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}
	// should be a single aggregated error string mentioning multiple issues
	msg := err.Error()
	expectSnippets := []string{
		"version must be 1",
		"targets[0].name is required",
		"format must be one of auto|yaml|toml|ini|json|raw",
		"output is required",
		"dedupe must be by_path|none",
		"newline must be \\n",
		"encoding must be utf8",
		"sources must not be empty",
	}
	for _, snip := range expectSnippets {
		if !strings.Contains(msg, snip) {
			t.Fatalf("validation message missing %q\nfull msg:\n%s", snip, msg)
		}
	}
}
