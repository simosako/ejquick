// Command mkzip creates a zip archive from a directory, mirroring the
// directory's own name as the archive root. Usage:
//
//	mkzip out.zip dir
//
// It exists so release archives can be built without the zip(1) binary.
package main

import (
	"archive/zip"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: mkzip <out.zip> <dir>")
		os.Exit(2)
	}
	outPath, dir := os.Args[1], os.Args[2]
	if err := run(outPath, dir); err != nil {
		fmt.Fprintf(os.Stderr, "mkzip: %v\n", err)
		os.Exit(1)
	}
}

func run(outPath, dir string) error {
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	root := filepath.Base(filepath.Clean(dir))
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(filepath.Dir(dir), path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		zf, err := w.Create(name)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = zf.Write(data)
		return err
	})
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	_ = root
	return nil
}
