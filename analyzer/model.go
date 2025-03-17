package analyzer

// SourceFile represents a source file with its path and dependencies
type SourceFile struct {
	Path       string `json:"path"`
	ImportPath string `json:"importPath"`
}

type ModuleLocalReplacement struct {
	OldPath string
	NewPath string
	Version string
}
