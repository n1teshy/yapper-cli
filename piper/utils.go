package piper

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
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
