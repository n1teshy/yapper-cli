package piper

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/term"
)

func HandleUnsupportedOS() {
	fmt.Println("Your platform is not yet supported.")
	fmt.Println("EXITING!")
	os.Exit(0)
}

func getTermWidth() (int, error) {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 0, err
	}
	return width, nil
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func getAppDir() (string, error) {
	var appDir string
	OS := runtime.GOOS

	if OS == OS_WINDOWS {
		appDir = os.Getenv("APPDATA")
	} else if OS == OS_LINUX {
		usr, err := user.Current()
		if err != nil {
			return "", err
		}
		appDir = filepath.Join(usr.HomeDir, ".config")
	} else if OS == OS_DARWIN {
		usr, err := user.Current()
		if err != nil {
			return "", err
		}
		appDir = filepath.Join(usr.HomeDir, "Library/Application Support")
	} else {
		HandleUnsupportedOS()
	}
	appDir = filepath.Join(appDir, APP_DIRNAME)
	return appDir, nil
}

func RandName(length int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	var sb strings.Builder

	for i := 0; i < length; i += 1 {
		idx := rand.Intn(len(letters))
		sb.WriteByte(letters[idx])
	}
	return sb.String()
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
	fmt.Print("\n")
	return err
}

func Unzip(src, dest string) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer reader.Close()

	os.MkdirAll(dest, 0755)

	for _, f := range reader.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		path := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
			continue
		}
		os.MkdirAll(filepath.Dir(path), f.Mode())
		wf, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		_, err = io.Copy(wf, rc)
		if err != nil {
			return err
		}
	}
	return nil
}

func UnpackTarGz(src, dest string) error {
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

func InstallPiper() error {
	var appDir string
	OS, arch := runtime.GOOS, runtime.GOARCH
	appDir, err := getAppDir()
	if err != nil {
		return err
	}
	exists, err := PathExists(filepath.Join(appDir, DIR_PIPER))
	if err != nil {
		return err
	}
	if exists {
		return nil
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
	// fmt.Printf("downloading zip %s into file %s\n", zipUrl, zipPath)
	Download(zipUrl, zipPath)

	if OS == OS_WINDOWS {
		Unzip(zipPath, appDir)
	} else {
		UnpackTarGz(zipPath, appDir)
	}

	os.Remove(zipPath)
	return nil
}

func makeVoiceFiles(langCode, voice, quality string) (string, string, error) {
	appDir, err := getAppDir()
	if err != nil {
		return "", "", err
	}
	voiceDir := filepath.Join(appDir, VOICE_DIRNAME)
	onnx_name := fmt.Sprintf("%s-%s-%s.onnx", langCode, voice, quality)
	onnxFile := filepath.Join(voiceDir, onnx_name)
	conf_name := fmt.Sprintf("%s-%s-%s.onnx.json", langCode, voice, quality)
	confFile := filepath.Join(voiceDir, conf_name)
	return onnxFile, confFile, nil
}

func DownloadVoice(langCode string, voice string, quality string) error {
	onnxFile, confFile, err := makeVoiceFiles(langCode, voice, quality)
	if err != nil {
		return err
	}
	onnxName, confName := filepath.Base(onnxFile), filepath.Base(confFile)
	os.MkdirAll(filepath.Dir(onnxFile), 0755)
	onnx_exists, err := PathExists(onnxFile)
	if err != nil {
		return err
	}
	conf_exists, err := PathExists(confFile)
	if err != nil {
		return err
	}
	if !onnx_exists || !conf_exists {
		fmt.Printf("Downloading requirements for %s(%s)...\n", voice, quality)
	}
	prefix := "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/"
	prefix += langCode
	if !onnx_exists {
		onnxUrl := fmt.Sprintf("%s/%s/%s/%s?download=true", prefix, voice, quality, onnxName)
		// fmt.Printf("downloading voice %s into file %s\n", onnxUrl, onnxFile)
		if err := Download(onnxUrl, onnxFile); err != nil {
			return err
		}
	}
	if !conf_exists {
		conf_url := fmt.Sprintf("%s/%s/%s/%s?download=true", prefix, voice, quality, confName)
		if err := Download(conf_url, confFile); err != nil {
			return err
		}
	}
	return nil
}

func DownloadVoiceMaps(update bool) error {
	appDir, err := getAppDir()
	if err != nil {
		return err
	}
	us_path := filepath.Join(appDir, "us_map.json")
	uk_path := filepath.Join(appDir, "uk_map.json")

	exists, err := PathExists(us_path)
	if err != nil {
		return err
	}
	if !exists || update {
		if err := Download(URL_US_VOICE_MAP, us_path); err != nil {
			return err
		}
	}

	exists, err = PathExists(uk_path)
	if err != nil {
		return err
	}
	if !exists || update {
		if err := Download(URL_UK_VOICE_MAP, uk_path); err != nil {
			return err
		}
	}
	return nil
}

func GetVoiceMap(accent string) (map[string][]string, error) {
	DownloadVoiceMaps(false)
	appDir, err := getAppDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(appDir, accent+"_map.json")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	jsonBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	var voiceMap map[string][]string
	err = json.Unmarshal(jsonBytes, &voiceMap)
	if err != nil {
		return nil, err
	}
	return voiceMap, nil
}

func TextToWave(text string, waveFile string) error {
	appDir, err := getAppDir()
	if err != nil {
		return err
	}

	exePath := filepath.Join(appDir, DIR_PIPER)
	if runtime.GOOS == OS_WINDOWS {
		exePath = filepath.Join(exePath, "piper.exe")
	} else {
		exePath = filepath.Join(exePath, DIR_PIPER)
	}

	onnxFile, confFile, err := makeVoiceFiles(LangCode, Voice, Quality)
	if err != nil {
		return err
	}
	cmd := exec.Command(exePath, "-m", onnxFile, "-c", confFile, "-f", waveFile, "--verbose")

	stdIn, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		defer stdIn.Close()
		io.WriteString(stdIn, text)
	}()

	// TODO: replace with cmd.Run()
	out, err := cmd.CombinedOutput()
	fmt.Println(string(out))
	if err != nil {
		return err
	}
	return nil
}
