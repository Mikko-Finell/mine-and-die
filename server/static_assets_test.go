package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveClientAssetsDirFromPrefersLocalClient(t *testing.T) {
	root := t.TempDir()
	clientDir := filepath.Join(root, "client")
	if err := os.MkdirAll(clientDir, 0o755); err != nil {
		t.Fatalf("failed to create client dir: %v", err)
	}

	resolved, ok := resolveClientAssetsDirFrom(root)
	if !ok {
		t.Fatalf("expected to resolve client dir under %s", root)
	}
	if resolved != clientDir {
		t.Fatalf("expected %s, got %s", clientDir, resolved)
	}
}

func TestResolveClientAssetsDirFromFallsBackToParent(t *testing.T) {
	workspace := t.TempDir()
	clientDir := filepath.Join(workspace, "client")
	if err := os.MkdirAll(clientDir, 0o755); err != nil {
		t.Fatalf("failed to create client dir: %v", err)
	}

	serverDir := filepath.Join(workspace, "server")
	if err := os.MkdirAll(serverDir, 0o755); err != nil {
		t.Fatalf("failed to create server dir: %v", err)
	}

	resolved, ok := resolveClientAssetsDirFrom(serverDir)
	if !ok {
		t.Fatalf("expected to resolve client dir from parent")
	}
	if resolved != clientDir {
		t.Fatalf("expected %s, got %s", clientDir, resolved)
	}
}

func TestResolveClientAssetsDirReturnsErrorWhenMissing(t *testing.T) {
	workspace := t.TempDir()
	if _, ok := resolveClientAssetsDirFrom(workspace); ok {
		t.Fatalf("expected resolution to fail when client dir missing")
	}
}
