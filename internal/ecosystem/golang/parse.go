package golang

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
)

var (
	errVersionNotFound  = errors.New("no semantic version declaration found")
	errVersionAmbiguous = errors.New("multiple semantic version declarations found")
	errVersionInjected  = errors.New("version declaration is build-time injected")
)

// parseError reports that a Go source file could not be parsed.
type parseError struct{ err error }

// Error returns the parse failure message.
func (e *parseError) Error() string { return fmt.Sprintf("parse Go source: %v", e.err) }

// Unwrap returns the underlying parser error.
func (e *parseError) Unwrap() error { return e.err }

// isParseError reports whether err came from the Go parser.
func isParseError(err error) bool {
	_, ok := errors.AsType[*parseError](err)
	return ok
}

// declaration describes the single editable version literal found in a Go file.
type declaration struct {
	Value      string
	Raw        string
	Start      int
	End        int
	Line       int
	TargetName string
	Indirect   bool
}

// parseContext carries the parsing state shared by the version resolution
// helpers while they descend into one file.
type parseContext struct {
	decls    map[*ast.ValueSpec]*ast.GenDecl
	comments ast.CommentMap
	fset     *token.FileSet
	filename string
}

// objectValue returns the initialiser expression for a variable or constant.
func objectValue(object *ast.Object) (ast.Expr, ast.Node) {
	switch decl := object.Decl.(type) {
	case *ast.ValueSpec:
		for i, name := range decl.Names {
			if name.Name != object.Name {
				continue
			}
			if len(decl.Values) == len(decl.Names) {
				return decl.Values[i], decl
			}
			return nil, decl
		}
	case *ast.AssignStmt:
		for i, lhs := range decl.Lhs {
			ident, ok := lhs.(*ast.Ident)
			if !ok || ident.Name != object.Name {
				continue
			}
			if len(decl.Rhs) == len(decl.Lhs) {
				return decl.Rhs[i], decl
			}
			return nil, decl
		}
	}
	return nil, nil
}

// hasInjectionDirective reports whether group is a build directive or a note
// that the value is injected at build time.
func hasInjectionDirective(group *ast.CommentGroup) bool {
	if group == nil {
		return false
	}
	markers := []string{
		"ldflags",
		"inject",
		"build time",
		"compile time",
		"set at build",
		"set by build",
		"set during build",
		"overridden at build",
	}
	for _, comment := range group.List {
		text := strings.TrimSpace(comment.Text)
		text = strings.TrimPrefix(text, "//")
		text = strings.TrimPrefix(text, "/*")
		text = strings.TrimSuffix(text, "*/")
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		lower := strings.ToLower(text)
		if strings.HasPrefix(lower, "go:") {
			return true
		}
		for _, marker := range markers {
			if strings.Contains(lower, marker) {
				return true
			}
		}
	}
	return false
}

// checkInjection rejects declarations whose comments mark them as
// injected by the build (//go: directives, -ldflags, and similar notes).
func checkInjection(object *ast.Object, ctx parseContext) error {
	if object == nil || object.Decl == nil {
		return nil
	}
	decl, ok := object.Decl.(ast.Node)
	if !ok {
		return nil
	}
	groups := append([]*ast.CommentGroup(nil), ctx.comments[decl]...)
	if spec, ok := decl.(*ast.ValueSpec); ok {
		if gen := ctx.decls[spec]; gen != nil {
			groups = append(groups, ctx.comments[gen]...)
		}
	}
	for _, group := range groups {
		if hasInjectionDirective(group) {
			return fmt.Errorf("%w: %s", errVersionInjected, object.Name)
		}
	}
	return nil
}

// resolveExpr follows an expression until it reaches a string literal.
func resolveExpr(expr ast.Expr, ctx parseContext, seen map[*ast.Object]bool) (*declaration, *ast.Object, error) {
	switch value := expr.(type) {
	case *ast.BasicLit:
		if value.Kind != token.STRING {
			return nil, nil, fmt.Errorf("version initialiser is not a string literal in %s", ctx.filename)
		}
		unquoted, err := strconv.Unquote(value.Value)
		if err != nil {
			return nil, nil, fmt.Errorf("decode version literal in %s: %w", ctx.filename, err)
		}
		if _, err := semver.NewVersion(unquoted); err != nil {
			return nil, nil, nil
		}
		position := ctx.fset.PositionFor(value.Pos(), false)
		end := ctx.fset.PositionFor(value.End(), false)
		return &declaration{
			Value: unquoted,
			Raw:   value.Value,
			Start: position.Offset,
			End:   end.Offset,
			Line:  position.Line,
		}, nil, nil
	case *ast.ParenExpr:
		return resolveExpr(value.X, ctx, seen)
	case *ast.Ident:
		object := value.Obj
		if object == nil {
			return nil, nil, fmt.Errorf("cannot resolve %s in %s; it may be declared in another file", value.Name, ctx.filename)
		}
		if object.Kind != ast.Var && object.Kind != ast.Con {
			return nil, nil, fmt.Errorf("%s in %s is not a variable or constant", value.Name, ctx.filename)
		}
		if seen[object] {
			return nil, nil, fmt.Errorf("version reference cycle involving %s in %s", value.Name, ctx.filename)
		}
		seen[object] = true
		next, _ := objectValue(object)
		if next == nil {
			if err := checkInjection(object, ctx); err != nil {
				return nil, nil, err
			}
			return nil, nil, fmt.Errorf("cannot find a literal initialiser for %s in %s", value.Name, ctx.filename)
		}
		found, target, err := resolveExpr(next, ctx, seen)
		if err != nil || found == nil {
			return found, target, err
		}
		if target == nil {
			target = object
		}
		return found, target, nil
	default:
		return nil, nil, fmt.Errorf("unsupported version initialiser %T in %s", expr, ctx.filename)
	}
}

// resolveObject resolves one Version declaration to its literal.
func resolveObject(object *ast.Object, ctx parseContext) (*declaration, error) {
	expr, _ := objectValue(object)
	if expr == nil {
		if err := checkInjection(object, ctx); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("cannot find a literal initialiser for %s in %s", object.Name, ctx.filename)
	}
	found, target, err := resolveExpr(expr, ctx, map[*ast.Object]bool{object: true})
	if err != nil || found == nil {
		return found, err
	}
	if target == nil {
		target = object
	}
	if err := checkInjection(target, ctx); err != nil {
		return nil, err
	}
	found.TargetName = target.Name
	found.Indirect = target != object
	return found, nil
}

// describeVersions renders candidates for the ambiguity error.
func describeVersions(candidates []*declaration) string {
	parts := make([]string, len(candidates))
	for i, candidate := range candidates {
		parts[i] = fmt.Sprintf("%s = %s (line %d)", candidate.TargetName, candidate.Value, candidate.Line)
	}
	return strings.Join(parts, ", ")
}

// parseDeclaration finds the one semantic-version declaration in a Go source
// file. It follows variable references within the same file.
func parseDeclaration(filename string, content []byte) (*declaration, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, content, parser.ParseComments)
	if err != nil {
		return nil, &parseError{err: err}
	}

	decls := make(map[*ast.ValueSpec]*ast.GenDecl)
	ast.Inspect(file, func(node ast.Node) bool {
		gen, ok := node.(*ast.GenDecl)
		if !ok {
			return true
		}
		for _, spec := range gen.Specs {
			if valueSpec, ok := spec.(*ast.ValueSpec); ok {
				decls[valueSpec] = gen
			}
		}
		return true
	})
	comments := ast.NewCommentMap(fset, file, file.Comments)
	ctx := parseContext{decls: decls, comments: comments, fset: fset, filename: filename}

	objects := make(map[*ast.Object]struct{})
	ast.Inspect(file, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if !ok || ident.Name != "Version" || ident.Obj == nil {
			return true
		}
		if ident.Obj.Kind != ast.Var && ident.Obj.Kind != ast.Con {
			return true
		}
		switch ident.Obj.Decl.(type) {
		case *ast.ValueSpec, *ast.AssignStmt:
			objects[ident.Obj] = struct{}{}
		}
		return true
	})
	if len(objects) == 0 {
		return nil, fmt.Errorf("%w in %s", errVersionNotFound, filename)
	}

	var (
		candidates       []*declaration
		resolutionErrors []error
	)
	for object := range objects {
		found, err := resolveObject(object, ctx)
		if err != nil {
			if errors.Is(err, errVersionInjected) {
				return nil, err
			}
			resolutionErrors = append(resolutionErrors, err)
			continue
		}
		if found != nil {
			candidates = append(candidates, found)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Line != candidates[j].Line {
			return candidates[i].Line < candidates[j].Line
		}
		return candidates[i].TargetName < candidates[j].TargetName
	})
	switch {
	case len(candidates) > 1:
		return nil, fmt.Errorf("%w in %s: %s", errVersionAmbiguous, filename, describeVersions(candidates))
	case len(candidates) == 1:
		return candidates[0], nil
	case len(resolutionErrors) > 0:
		return nil, resolutionErrors[0]
	default:
		return nil, fmt.Errorf("%w in %s (%d Version declaration(s))", errVersionNotFound, filename, len(objects))
	}
}
