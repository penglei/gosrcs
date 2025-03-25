package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gosrcs/analyzer"
)

func listSources(pkgDir string, buildFlags []string) ([]analyzer.SourceFile, error) {
	modDir, err := findGoModDir(pkgDir)
	if err != nil {
		return nil, err
	}

	pkgs, err := analyzer.LoadPackages(modDir, pkgDir, buildFlags)
	if err != nil {
		return nil, err
	}

	m, err := analyzer.NewModule(modDir)
	if err != nil {
		return nil, err
	}
	ac := &analyzer.AnalyzeContext{
		BaseDir:    m.ModDir,
		MainModule: m,
		Cache:      make(analyzer.ModuleCache),
		Files:      make([]analyzer.SourceFile, 0),
	}

	if err = m.AddGoModFile(ac); err != nil {
		return nil, err
	}

	for _, pkg := range pkgs {
		if err = m.ProcessPackage(pkg, ac); err != nil {
			return nil, err
		}
	}
	return ac.Files, nil
}

func findGoModDir(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found in any parent directory")
		}
		dir = parent
	}
}
