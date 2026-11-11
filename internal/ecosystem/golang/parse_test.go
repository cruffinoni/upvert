package golang

import (
	"errors"
	"testing"
)

func TestParseGoVersionDirect(t *testing.T) {
	content := "package demo\n\nconst Version = \"1.2.3\"\n"
	found, err := parseDeclaration("version.go", []byte(content))
	if err != nil {
		t.Fatalf("parseDeclaration: %v", err)
	}
	if found.Value != "1.2.3" || found.TargetName != "Version" || found.Indirect {
		t.Fatalf("found = %+v", found)
	}
}

func TestParseGoVersionIndirectChain(t *testing.T) {
	content := "package demo\n\nvar versionString = \"1.2.3\"\nvar buildVersion = versionString\nconst Version = buildVersion\n"
	found, err := parseDeclaration("version.go", []byte(content))
	if err != nil {
		t.Fatalf("parseDeclaration: %v", err)
	}
	if found.Value != "1.2.3" || found.TargetName != "versionString" || !found.Indirect {
		t.Fatalf("found = %+v", found)
	}
}

func TestParseGoVersionMultipleSemver(t *testing.T) {
	content := "package demo\n\nconst Version = \"1.2.3\"\n\nfunc other() {\n\tconst Version = \"1.2.4\"\n}\n"
	_, err := parseDeclaration("version.go", []byte(content))
	if !errors.Is(err, errVersionAmbiguous) {
		t.Fatalf("error = %v, want errVersionAmbiguous", err)
	}
}

func TestParseGoVersionUsesOnlySemverDeclaration(t *testing.T) {
	content := "package demo\n\nconst Version = \"dev\"\n\nfunc other() {\n\tconst Version = \"1.2.4\"\n}\n"
	found, err := parseDeclaration("version.go", []byte(content))
	if err != nil {
		t.Fatalf("parseDeclaration: %v", err)
	}
	if found.Value != "1.2.4" || found.TargetName != "Version" {
		t.Fatalf("found = %+v", found)
	}
}

func TestParseGoVersionRejectsInjectedVariable(t *testing.T) {
	content := "package demo\n\n//go:embed version.txt\nvar buildVersion string\nconst Version = buildVersion\n"
	_, err := parseDeclaration("version.go", []byte(content))
	if !errors.Is(err, errVersionInjected) {
		t.Fatalf("error = %v, want errVersionInjected", err)
	}
}

func TestParseGoVersionReportsParseFailure(t *testing.T) {
	_, err := parseDeclaration("version.go", []byte("package demo\nconst Version = \n"))
	if !isParseError(err) {
		t.Fatalf("error = %v, want parse failure", err)
	}
}

func TestParseGoVersionAllowsNormalTrailingComment(t *testing.T) {
	content := "package demo\n\nvar buildVersion = \"1.0.0\" // current version\nconst Version = buildVersion\n"
	found, err := parseDeclaration("version.go", []byte(content))
	if err != nil {
		t.Fatalf("parseDeclaration: %v", err)
	}
	if found.Value != "1.0.0" || found.TargetName != "buildVersion" {
		t.Fatalf("found = %+v", found)
	}
}

func TestParseGoVersionRejectsTrailingInjectionComment(t *testing.T) {
	content := "package demo\n\nvar buildVersion = \"1.0.0\" // set via -ldflags\nconst Version = buildVersion\n"
	_, err := parseDeclaration("version.go", []byte(content))
	if !errors.Is(err, errVersionInjected) {
		t.Fatalf("error = %v, want errVersionInjected", err)
	}
}
