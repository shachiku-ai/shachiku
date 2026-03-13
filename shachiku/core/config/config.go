package config

import (
	"os"
	"path/filepath"
)

// GetDataDir returns the base data directory for shachiku.
// By default, it maps to ~/.shachiku/data
func GetDataDir() string {
	dataDir := os.Getenv("SHACHIKU_DATA_DIR")
	if dataDir != "" {
		return dataDir
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "data"
	}
	return filepath.Join(homeDir, ".shachiku", "data")
}

// GetCertDir returns the base directory for shachiku certificates.
// By default, it maps to ~/.cert/shachiku
func GetCertDir() string {
	certDir := os.Getenv("SHACHIKU_CERT_DIR")
	if certDir != "" {
		return certDir
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(GetDataDir(), "certs")
	}
	return filepath.Join(homeDir, ".cert", "shachiku")
}
