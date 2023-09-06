package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

var blobFilePermission fs.FileMode = 0644

// Pull pulls the OCI artifacts at the OCI Image reference and downloads them to the specified dir.
func Pull(ctx context.Context, image string, username string, password string, outputDir string, plainHTTP bool) error {
	// Parse image into chunks.
	chunks := strings.Split(image, "/")
	subChunks := strings.Split(chunks[2], ":")

	registryName := chunks[0]
	repositoryName := chunks[1] + "/" + subChunks[0]
	imageReference := subChunks[1]

	repo, err := remote.NewRepository(fmt.Sprintf("%s/%s", registryName, repositoryName))
	if err != nil {
		return fmt.Errorf("error setting up repository: %w", err)
	}

	// Setup the client credentials.
	repo.Client = &auth.Client{
		Credential: func(ctx context.Context, reg string) (auth.Credential, error) {
			return auth.Credential{
				Username: username,
				Password: password,
			}, nil
		},
	}

	if plainHTTP {
		repo.PlainHTTP = true
	}

	log.Printf("pulling blobs from %s/%s:%s\n", registryName, repositoryName, imageReference)

	// Obtains the manifest descriptor for the specified imageReference.
	manifestDescriptor, rc, err := repo.FetchReference(ctx, imageReference)
	if err != nil {
		return fmt.Errorf("error fetcing oci reference manifest: %w", err)
	}
	defer rc.Close()

	// Read the bytes of the manifest descriptor from the io.ReadCloser.
	pulledContent, err := content.ReadAll(rc, manifestDescriptor)
	if err != nil {
		return fmt.Errorf("error reading oci reference manifest: %w", err)
	}

	// JSON Decodes the bytes read into an OCI Manifest.
	var pulledManifest ocispec.Manifest
	if err := json.Unmarshal(pulledContent, &pulledManifest); err != nil {
		return fmt.Errorf("failed to json decode the pulled oci manifest: %w", err)
	}

	// Check if the Output Path exists before writing to it.
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		return fmt.Errorf("specified output path '%s' does not exits: %w", outputDir, err)
	}

	// Loop over the layers found in the OCI Manifest to download their content (blob).
	for _, layer := range pulledManifest.Layers {
		filename := layer.Annotations["org.opencontainers.image.title"]

		log.Printf("downloading blob %s [%s]\n", filename, byteCountIEC(layer.Size))

		pulledBlob, err := content.FetchAll(ctx, repo, layer)
		if err != nil {
			return fmt.Errorf("failed to fetch layer content (blob): %w", err)
		}

		dest := path.Join(outputDir, filename)
		if err := os.WriteFile(dest, pulledBlob, blobFilePermission); err != nil {
			return fmt.Errorf("failed to write layer content (blob) to file %s: %w", dest, err)
		}
	}

	return nil
}

func byteCountIEC(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}

	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}
