// Command mkzip creates a zip archive from a directory, mirroring the
// directory's own name as the archive root. Usage:
//
//	mkzip out.zip dir
//
// It exists so release archives can be built without the zip(1) binary.
package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
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
	dir = filepath.Clean(dir)
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("stat input directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("input is not a directory: %s", dir)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	keepOutput := false
	defer func() {
		if !keepOutput {
			_ = os.Remove(outPath)
		}
	}()

	w := zip.NewWriter(f)
	root := filepath.Base(dir)
	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(filepath.Join(root, rel))
		zf, err := w.Create(name)
		if err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(zf, src)
		closeErr := src.Close()
		return errors.Join(copyErr, closeErr)
	})
	zipCloseErr := w.Close()
	fileCloseErr := f.Close()
	if err := errors.Join(
		walkErr,
		wrapError("close zip", zipCloseErr),
		wrapError("close output", fileCloseErr),
	); err != nil {
		return err
	}
	keepOutput = true
	return nil
}

func wrapError(action string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", action, err)
}
