// +build !solution

package tarstream

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
)

func Send(dir string, w io.Writer) error {
	tw := tar.NewWriter(w)
	defer tw.Close()

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
			return err
		}

		if !info.Mode().IsRegular() && !info.IsDir() {
			return nil
		}

		filename, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		hdr, err := tar.FileInfoHeader(info, info.Name())
		hdr.Name = filename

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if !info.IsDir() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()

			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		}

		return nil
	})
}

func Receive(dir string, r io.Reader) error {
	tr := tar.NewReader(r)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		filenameWithPath := path.Join(dir, hdr.Name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if _, err := os.Stat(filenameWithPath); err != nil {
				if err := os.MkdirAll(filenameWithPath, os.FileMode(hdr.Mode)); err != nil {
					return err
				}
			}
		case tar.TypeReg:
			f, err := os.OpenFile(path.Join(dir, hdr.Name), os.O_CREATE|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			defer f.Close()

			if _, err := io.Copy(f, tr); err != nil {
				return err
			}
		}
	}
	return nil
}
