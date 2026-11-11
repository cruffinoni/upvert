// Package project detects the version-bearing layout of a repository by
// applying the language strategy registry to a project file snapshot.
package project

import (
	"github.com/dayrion/upvert/internal/ecosystem"
)

// Part is one language's contribution to a project.
type Part struct {
	// Target is the language's version declarations.
	Target ecosystem.Target
	// Language is the strategy that owns the target.
	Language ecosystem.Language
}

// Layout describes the detected layout of a repository.
type Layout struct {
	// Kind identifies the project ecosystem, or ecosystem.Multi.
	Kind ecosystem.Kind
	// Parts lists one entry per contributing language.
	Parts []Part
}

// IsMulti reports whether the project mixes several ecosystems.
func (p *Layout) IsMulti() bool {
	return p.Kind == ecosystem.Multi
}

// UsesTags reports whether the project reads its version from git tags.
func (p *Layout) UsesTags() bool {
	if p.IsMulti() {
		return true
	}
	for _, part := range p.Parts {
		if part.Target.TagSource {
			return true
		}
	}
	return false
}

// Files returns every version-bearing file in report and stage order.
func (p *Layout) Files() []ecosystem.File {
	var files []ecosystem.File
	for _, part := range p.Parts {
		files = append(files, part.Target.Files...)
	}
	return files
}

// Primary returns the first version-bearing file, or the zero File when the
// project has none.
func (p *Layout) Primary() ecosystem.File {
	if len(p.Parts) == 0 || len(p.Parts[0].Target.Files) == 0 {
		return ecosystem.File{}
	}
	return p.Parts[0].Target.Files[0]
}
