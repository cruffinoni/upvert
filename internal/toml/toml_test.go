package toml

import "testing"

func TestUpdatePreservesFormatting(t *testing.T) {
	initial := []byte("# head\n[project]\nname = 'demo'\nversion = '1.0.0' # keep\n\n[tool.poetry.dependencies]\nversion = '2.0.0'\n")
	want := "# head\n[project]\nname = 'demo'\nversion = '1.1.0' # keep\n\n[tool.poetry.dependencies]\nversion = '2.0.0'\n"
	updated, err := Update(initial, [][]string{{"project"}}, "1.1.0")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if string(updated) != want {
		t.Fatalf("content = %q, want %q", updated, want)
	}
}

func TestUpdateMultipleSections(t *testing.T) {
	initial := []byte("[project]\nversion = \"1.0.0\"\n\n[tool.poetry]\nversion = \"1.0.0\"\n")
	want := "[project]\nversion = \"1.1.0\"\n\n[tool.poetry]\nversion = \"1.1.0\"\n"
	updated, err := Update(initial, [][]string{{"project"}, {"tool", "poetry"}}, "1.1.0")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if string(updated) != want {
		t.Fatalf("content = %q, want %q", updated, want)
	}
}

func TestDecodeAndQueries(t *testing.T) {
	document, err := Decode("[project]\nversion = \"1.2.3\"\n")
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got := String(document, "project", "version"); got != "1.2.3" {
		t.Fatalf("String = %q, want 1.2.3", got)
	}
	if !TableExists(document, "project") {
		t.Fatal("TableExists = false, want true")
	}
}
