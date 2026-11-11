package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

const (
	// maxBinaryBytes bounds the extracted upvert executable.
	maxBinaryBytes = 128 << 20
)

// readLimited reads a binary and rejects content larger than maxBinaryBytes.
func readLimited(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxBinaryBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBinaryBytes {
		return nil, fmt.Errorf("%w: binary exceeds %d bytes", ErrTooLarge, maxBinaryBytes)
	}
	return data, nil
}

// extractTarGz returns the upvert binary stored in a tar.gz archive.
func extractTarGz(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", filePath, err)
	}
	defer file.Close()

	compressed, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filePath, err)
	}
	defer compressed.Close()

	reader := tar.NewReader(compressed)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", filePath, err)
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}
		if path.Base(header.Name) != "upvert" {
			continue
		}
		if header.Size > maxBinaryBytes {
			return nil, &ArchiveError{Archive: filePath, Entry: header.Name, Err: ErrTooLarge}
		}
		data, err := readLimited(reader)
		if err != nil {
			return nil, fmt.Errorf("read %s from %s: %w", header.Name, filePath, err)
		}
		return data, nil
	}
	return nil, &ArchiveError{Archive: filePath, Entry: "upvert", Err: ErrBinaryNotFound}
}

// extractZip returns the upvert.exe binary stored in a zip archive.
func extractZip(filePath string) ([]byte, error) {
	archive, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filePath, err)
	}
	defer archive.Close()

	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() || path.Base(entry.Name) != "upvert.exe" {
			continue
		}
		if entry.UncompressedSize64 > maxBinaryBytes {
			return nil, &ArchiveError{Archive: filePath, Entry: entry.Name, Err: ErrTooLarge}
		}
		reader, err := entry.Open()
		if err != nil {
			return nil, fmt.Errorf("read %s from %s: %w", entry.Name, filePath, err)
		}
		data, readErr := readLimited(reader)
		closeErr := reader.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read %s from %s: %w", entry.Name, filePath, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close %s: %w", entry.Name, closeErr)
		}
		return data, nil
	}
	return nil, &ArchiveError{Archive: filePath, Entry: "upvert.exe", Err: ErrBinaryNotFound}
}

// extractBinary returns the upvert executable from the release archive.
func extractBinary(filePath, asset string) ([]byte, error) {
	var (
		data []byte
		err  error
	)
	switch {
	case strings.HasSuffix(asset, ".zip"):
		data, err = extractZip(filePath)
	case strings.HasSuffix(asset, ".tar.gz"):
		data, err = extractTarGz(filePath)
	default:
		return nil, &ArchiveError{Archive: asset, Err: ErrUnsupportedArchive}
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, &ArchiveError{Archive: filePath, Err: ErrEmptyBinary}
	}
	return data, nil
}
