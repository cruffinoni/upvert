package bump

import (
	"fmt"
	"strings"

	"github.com/dayrion/upvert/internal/ecosystem"
	"github.com/dayrion/upvert/internal/fileio"
	"github.com/dayrion/upvert/internal/semver"
)

// printSummary renders the dry-run report.
func (s *service) printSummary(current, next string, kind semver.Kind, latest string) {
	out := s.cfg.Stdout
	row := func(label, value string) {
		_, _ = fmt.Fprintf(out, "  %-22s %s\n", label, value)
	}
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "─────────────────────────────────────────────")
	_, _ = fmt.Fprintln(out, "  Summary")
	_, _ = fmt.Fprintln(out, "─────────────────────────────────────────────")
	row("Project type:", string(s.proj.Kind))
	if s.proj.UsesTags() {
		row("Version source:", "Git tags (vX.Y.Z)")
		files := s.proj.Files()
		if len(files) == 0 {
			row("Version files:", "none")
		} else {
			paths := make([]string, len(files))
			for i, file := range files {
				paths[i] = file.Path
			}
			row("Version files:", strings.Join(paths, ", "))
		}
	} else {
		row("Version file:", s.proj.Primary().Path)
	}
	if !s.proj.IsMulti() && s.proj.Kind == ecosystem.Python {
		var modules []string
		for i, file := range s.proj.Files() {
			if i == 0 {
				continue
			}
			if file.InCode {
				modules = append(modules, file.Path)
			}
		}
		if len(modules) > 0 {
			row("Version module(s):", strings.Join(modules, ", "))
		}
	}
	if latest == "" {
		row("Latest tag:", "none")
	} else {
		row("Latest tag:", latest)
	}
	row("Current version:", current)
	row("New version:", next)
	row("Bump type:", string(kind))
	row("Action:", s.cfg.Mode.String())
	if s.cfg.PreRelease != "" {
		row("Pre-release:", s.cfg.PreRelease)
	}
	if s.cfg.NoChangelog {
		row("Changelog:", "skip")
	} else {
		row("Changelog:", s.cfg.ChangelogFile)
	}
	if s.cfg.Mode == ModeWrite {
		row("Git tag:", "skip")
		row("Git commit:", "skip")
	} else {
		row("Git tag:", "v"+next)
		row("Git commit:", strings.ReplaceAll(s.cfg.Message, versionPlaceholder, next))
	}
	if s.cfg.Mode == ModePush {
		row("Push to origin:", "yes")
	} else {
		row("Push to origin:", "no")
	}
	if fileio.Exists(fileio.Resolve(s.root, "go.mod")) {
		row("Go mod tidy:", "yes")
	}
	_, _ = fmt.Fprintln(out, "─────────────────────────────────────────────")
}
