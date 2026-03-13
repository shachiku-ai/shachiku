package channel

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"shachiku/core/config"
	"time"
)

func downloadFileToTmp(url, filename string) (string, error) {
	tmpDir := filepath.Join(config.GetDataDir(), "tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}

	// Make sure filename is not empty and strip path components just in case
	filename = filepath.Base(filename)
	if filename == "." || filename == "" {
		filename = "downloaded_file"
	}

	ext := filepath.Ext(filename)
	base := filename[:len(filename)-len(ext)]
	newFileName := fmt.Sprintf("%s_%d%s", base, time.Now().Unix(), ext)
	outPath := filepath.Join(tmpDir, newFileName)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", err
	}

	return outPath, nil
}
