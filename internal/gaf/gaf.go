package gaf

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
)

// ErrMaformedGaf denotes the error for failed extraction of a gaf
// due to it being malformed.
var ErrMaformedGaf = errors.New("unable to extract malformed gaf")

// ReadClosers holds ReadClosers for the content of the gaf file.
type ReadClosers struct {
	MBRRC  io.ReadCloser
	BootRC io.ReadCloser
	RootRC io.ReadCloser
	SBOMRC io.ReadCloser
}

const (
	MBR  string = "mbr.img"
	Boot string = "boot.img"
	Root string = "root.img"
	SBOM string = "sbom.json"
)

// Extract extracts the content of a gaf archive.
func Extract(ctx context.Context, source io.ReaderAt, size int64) (ReadClosers, error) {
	gafRCs := ReadClosers{}

	reader, err := zip.NewReader(source, size)
	if err != nil {
		return gafRCs, fmt.Errorf("error reading from zip reader: %w", err)
	}

	// Check the gaf archive contains the files described in the gaf spec.
	for _, file := range reader.File {
		switch file.Name {
		case MBR:
			reader, err := file.Open()
			if err != nil {
				return ReadClosers{}, err
			}
			gafRCs.MBRRC = reader

		case Boot:
			reader, err := file.Open()
			if err != nil {
				return ReadClosers{}, err
			}
			gafRCs.BootRC = reader

		case Root:
			reader, err := file.Open()
			if err != nil {
				return ReadClosers{}, err
			}
			gafRCs.RootRC = reader

		case SBOM:
			reader, err := file.Open()
			if err != nil {
				return ReadClosers{}, err
			}
			gafRCs.SBOMRC = reader
		}
	}

	if gafRCs.MBRRC == nil ||
		gafRCs.BootRC == nil ||
		gafRCs.RootRC == nil ||
		gafRCs.SBOMRC == nil {
		return ReadClosers{}, ErrMaformedGaf
	}

	// Unzip archive to readers.
	return gafRCs, nil
}
