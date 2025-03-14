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
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/term"
)

const OS_WINDOWS = "windows"
const OS_LINUX = "linux"
const OS_DARWIN = "darwin"

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

func handleUnsupportedOS() {
	fmt.Println("Your platform is not yet supported.")
	fmt.Println("EXITING!")
	os.Exit(0)
}

func InstallPiper() error {
	var appDir string
	OS, arch := runtime.GOOS, runtime.GOARCH

	if OS == OS_WINDOWS {
		appDir = os.Getenv("APPDATA")
	} else if OS == OS_LINUX {
		usr, err := user.Current()
		if err != nil {
			return err
		}
		appDir = filepath.Join(usr.HomeDir, ".config")
	} else if OS == OS_DARWIN {
		usr, err := user.Current()
		if err != nil {
			return err
		}
		appDir = filepath.Join(usr.HomeDir, "Library/Application Support")
	} else {
		handleUnsupportedOS()
	}
	appDir = filepath.Join(appDir, "yapper-cli")
	_, err := os.Stat(filepath.Join(appDir, "piper"))
	if err == nil || !os.IsNotExist(err) {
		return err
	}
	os.MkdirAll(appDir, 0755)

	fmt.Println("installing piper...")
	zipUrl := "https://github.com/rhasspy/piper/releases/download/2023.11.14-2"
	zipPath := filepath.Join(appDir, "piper.zip")

	if OS == OS_LINUX {
		if arch == "aarch64" || arch == "arm64" {
			zipUrl += "/piper_linux_aarch64.tar.gz"
		} else if arch == "armv7l" || arch == "armv7" {
			zipUrl += "/piper_linux_armv7l.tar.gz"
		} else {
			zipUrl += "/piper_linux_x86_64.tar.gz"
		}
	} else if OS == OS_WINDOWS {
		zipUrl += "/piper_windows_amd64.zip"
	} else {
		zipUrl += "/piper_macos_x64.tar.gz"
	}

	Download(zipUrl, zipPath)

	if OS == OS_WINDOWS {
		Unzip(zipPath, appDir)
	} else {
		UnpackTarGz(zipPath, appDir)
	}

	os.Remove(zipPath)
	return nil
}
