package providers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveReplayFileKeepsFilesInsideConfiguredDirectory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveReplayFile(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != path {
		t.Fatalf("resolved = %q, quiere %q", resolved, path)
	}
	if _, err := resolveReplayFile(path, ".."+string(filepath.Separator)+"outside.json"); err == nil {
		t.Fatal("se permitió salir del directorio de replay")
	}
}

func TestResolveReplayFileRejectsMissingAndNonRegularFiles(t *testing.T) {
	root := t.TempDir()
	configured := filepath.Join(root, "missing.json")
	if _, err := resolveReplayFile(configured, "missing.json"); err == nil {
		t.Fatal("se aceptó un archivo inexistente")
	}
	if _, err := resolveReplayFile(root, ""); err == nil {
		t.Fatal("se aceptó un directorio como replay")
	}
}

func TestResolveReplayFileRejectsPathSeparators(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, requested := range []string{filepath.Join("nested", "sample.json"), path} {
		if _, err := resolveReplayFile(path, requested); err == nil {
			t.Errorf("se aceptó la ruta %q", requested)
		}
	}
}
