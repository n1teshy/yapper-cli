package piper

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

func getTermWidth() (int, error) {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 0, err
	}
	return width, nil
}

func Download(url string, filename string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	if resp.ContentLength == -1 {
		_, err = io.Copy(out, resp.Body)
		return err
	}

	termWidth, err := getTermWidth()
	if err != nil {
		return err
	}

	termWidth = int(math.Min(float64(termWidth), 80))
	hook := func(pw *ProgressWriter) {
		progress := float32(pw.Current) / float32(pw.Total)
		barLen := int(float32(termWidth-2) * progress)
		if barLen > pw.lastBarLen {
			padLen := termWidth - 2 - barLen
			fmt.Printf("\r[%s%s]", strings.Repeat("=", barLen), strings.Repeat(" ", padLen))
			pw.lastBarLen = barLen
		}
	}
	progress := &ProgressWriter{
		Total: resp.ContentLength,
		Hook:  hook,
	}
	_, err = io.Copy(out, io.TeeReader(resp.Body, progress))
	return err
}

func Unzip(src string, dest string) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, f := range reader.File {
		path := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
			continue
		}
		rf, err := f.Open()
		if err != nil {
			return err
		}
		defer rf.Close()
		wf, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		_, err = io.Copy(wf, rf)
		if err != nil {
			return err
		}
	}
	return nil
}

func UnpackTarGz(src string, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		tgtPath := filepath.Join(dest, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(tgtPath, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(tgtPath), 0755); err != nil {
				return err
			}

			outFile, err := os.OpenFile(tgtPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		case tar.TypeSymlink:
			if err := os.Symlink(header.Linkname, tgtPath); err != nil {
				return err
			}
		default:

		}
	}
	return nil
}
