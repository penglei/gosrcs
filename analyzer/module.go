package analyzer

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"
)

func LoadPackages(modDir, pkgDir string, buildFlags []string) ([]*packages.Package, error) {
	// Configure package loading
	config := &packages.Config{
		Mode:       packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedModule | packages.NeedEmbedFiles,
		Dir:        modDir,
		BuildFlags: buildFlags,
	}

	// Load the package and its dependencies
	pkgs, err := packages.Load(config, pkgDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load package: %w", err)
	}

	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages found for dir: %s", pkgDir)
	}

	if len(pkgs[0].Errors) > 0 {
		return nil, fmt.Errorf("errors loading package: %v", pkgs[0].Errors)
	}
	return pkgs, nil
}

type ModuleCache = map[string]*GoModule

type AnalyzeContext struct {
	BaseDir    string
	MainModule *GoModule
	Cache      ModuleCache
	Files      []SourceFile
}

// GoModule handles package analysis
type GoModule struct {
	ModDir        string //absolute go module path
	ModPath       string
	modFile       *modfile.File
	localReplace  map[string]ModuleLocalReplacement
	pkgsProcessed map[string]bool
}

// NewModule creates a new GoModule
func NewModule(modDir string) (*GoModule, error) {
	modFilePath := filepath.Join(modDir, "go.mod")

	// Read go.mod file
	modFileContent, err := os.ReadFile(modFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.mod file: %w", err)
	}

	// Parse go.mod file
	modFile, err := modfile.Parse(modFilePath, modFileContent, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse go.mod file: %w", err)
	}

	// Extract replacements
	var localReplace = make(map[string]ModuleLocalReplacement)
	for _, replace := range modFile.Replace {
		// If the replacement path is not a module path (doesn't contain a version),
		// it's likely a local directory
		if !filepath.IsAbs(replace.New.Path) && replace.New.Version == "" {
			replacement := ModuleLocalReplacement{
				OldPath: replace.Old.Path,
				NewPath: filepath.Join(modDir, replace.New.Path),
			}
			localReplace[replace.Old.Path] = replacement
		}
	}

	return &GoModule{
		ModDir:        modDir,
		ModPath:       modFile.Module.Mod.Path,
		modFile:       modFile,
		localReplace:  localReplace,
		pkgsProcessed: make(map[string]bool),
	}, nil
}

func (m *GoModule) AddGoModFile(ac *AnalyzeContext) error {
	var sources []SourceFile
	var files = []string{"go.mod", "go.sum"}

	for _, file := range files {
		fileRelPath, err := filepath.Rel(ac.BaseDir, path.Join(m.ModDir, file))
		if err != nil {
			return err
		}
		sourceFile := SourceFile{
			Path: fileRelPath,
		}
		sources = append(sources, sourceFile)
	}
	ac.Files = append(ac.Files, sources...)
	return nil
}

// ProcessPackage processes a single package and its dependencies
func (m *GoModule) ProcessPackage(pkg *packages.Package, ac *AnalyzeContext) error {

	var sources []SourceFile
	if m.pkgsProcessed[pkg.PkgPath] {
		return nil
	}
	m.pkgsProcessed[pkg.PkgPath] = true

	var files []string
	files = append(files, pkg.GoFiles...)
	files = append(files, pkg.OtherFiles...)
	files = append(files, pkg.EmbedFiles...)
	// Add source files
	for _, file := range files {
		// Convert to relative path if the package is within the module
		relPath, err := filepath.Rel(m.ModDir, file)
		if err == nil && !strings.HasPrefix(relPath, "..") {
			file = relPath
		}

		fileRelPath, err := filepath.Rel(ac.BaseDir, path.Join(pkg.Module.Dir, file))
		if err != nil {
			return err
		}
		sourceFile := SourceFile{
			Path:       fileRelPath,
			ImportPath: pkg.PkgPath,
		}
		sources = append(sources, sourceFile)
	}

	//Process package dependencies(imports)
	for pkgPath, importPkg := range pkg.Imports {
		if m.pkgsProcessed[pkgPath] {
			continue
		}
		//not module(std library)
		if importPkg.Module == nil {
			continue
		}

		if importPkg.Module.Path == m.ModPath {
			if err := m.ProcessPackage(importPkg, ac); err != nil {
				return err
			}
		} else {
			var err error
			depModPath := importPkg.Module.Path
			_, localReplaced := m.localReplace[depModPath]
			if localReplaced {
				localModule, parsed := ac.Cache[depModPath]
				if !parsed {
					localModule, err = NewModule(importPkg.Module.Dir)
					localModule.localReplace = m.localReplace //replace must be applied to dependencies.
					if err != nil {
						return fmt.Errorf("failed to parse local module: %w", err)
					}
					if err = localModule.AddGoModFile(ac); err != nil {
						return err
					}
					ac.Cache[depModPath] = localModule
				}

				if err = localModule.ProcessPackage(importPkg, ac); err != nil {
					return fmt.Errorf("failed to analyze package(%s) in local module: %w", importPkg.PkgPath, err)
				}
			}
		}
	}

	ac.Files = append(ac.Files, sources...)
	return nil
}
