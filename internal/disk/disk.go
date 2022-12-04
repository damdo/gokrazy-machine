package disk

import (
	"fmt"
	"io"
	"os"

	"github.com/gokrazy/tools/packer"
)

const mb = 1024 * 1024

const (
	// MBRPartitionOffset is the offset where to find the MBR partition.
	MBRPartitionOffset = 0

	// BootPartitionOffset is the offset where to find the Boot partition.
	BootPartitionOffset = 8192 * 512

	// RootPartitionOffset is the offset where to find the first Root partition.
	RootPartitionOffset = BootPartitionOffset + 100*mb
)

// PartsToFull merges multi parts (mbr, boot, root) image files of a disk into a single disk image file.
func PartsToFull(mbrSourcePath, bootSourcePath, rootSourcePath, destPath string) error {
	// TODO: read this from the source imgs if possible
	var targetStorageBytes = 2147483648

	// TODO: read hostname from source root img
	p := packer.NewPackForHost("gokrazy")

	// TODO: read these from the source imgs if possible
	p.UsePartuuid = true
	p.UseGPTPartuuid = true
	p.UseGPT = true

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("error creating destination disk file %s: %w", destPath, err)
	}

	if err := f.Truncate(int64(targetStorageBytes)); err != nil {
		return fmt.Errorf("error preparing disk file: %w", err)
	}

	if err := p.Partition(f, uint64(targetStorageBytes)); err != nil {
		return fmt.Errorf("error partitioning disk file: %w", err)
	}

	if _, err := f.Seek(BootPartitionOffset, io.SeekStart); err != nil {
		return fmt.Errorf("error seeking disk boot partition start: %w", err)
	}

	bootFile, err := os.Open(bootSourcePath)
	if err != nil {
		return fmt.Errorf("error opening boot partition file %s: %w", bootSourcePath, err)
	}

	if _, err := io.Copy(f, bootFile); err != nil {
		return fmt.Errorf("error writing boot partition to disk file: %w", err)
	}

	if _, err := f.Seek(MBRPartitionOffset, io.SeekStart); err != nil {
		return fmt.Errorf("error seeking mbr partition start: %w", err)
	}

	mbrFile, err := os.Open(mbrSourcePath)
	if err != nil {
		return fmt.Errorf("error opening mbr partition file %s: %w", mbrSourcePath, err)
	}

	if _, err := io.Copy(f, mbrFile); err != nil {
		return fmt.Errorf("error writing mbr partition to disk file: %w", err)
	}

	if _, err := f.Seek(RootPartitionOffset, io.SeekStart); err != nil {
		return fmt.Errorf("error seeking disk file: %w", err)
	}

	tmp, err := os.CreateTemp("", "gokr-packer")
	if err != nil {
		return fmt.Errorf("error creating temporary file: %w", err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	rootFile, err := os.Open(rootSourcePath)
	if err != nil {
		return fmt.Errorf("error opening root partition file %s: %w", rootSourcePath, err)
	}

	if _, err := io.Copy(f, rootFile); err != nil {
		return fmt.Errorf("error writing root partition to disk file: %w", err)
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("error seeking temporary file: %w", err)
	}

	if _, err := io.Copy(f, tmp); err != nil {
		return fmt.Errorf("error writing temporary file to disk file: %w", err)
	}

	return nil
}
