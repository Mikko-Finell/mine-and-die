package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func resolveClientAssetsDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve client assets: %w", err)
	}
	if dir, ok := resolveClientAssetsDirFrom(cwd); ok {
		return dir, nil
	}
	exePath, err := os.Executable()
	if err == nil {
		base := filepath.Dir(exePath)
		if dir, ok := resolveClientAssetsDirFrom(base); ok {
			return dir, nil
		}
	}
	return "", fmt.Errorf("client assets directory not found")
}

func resolveClientAssetsDirFrom(base string) (string, bool) {
	candidates := []string{
		filepath.Join(base, "client"),
		filepath.Join(base, "..", "client"),
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if info.IsDir() {
			abs, err := filepath.Abs(candidate)
			if err != nil {
				continue
			}
			return abs, true
		}
	}
	return "", false
}
