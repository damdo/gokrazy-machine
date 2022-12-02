package cmd

import (
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/gokrazy/tools/packer"
)

func createFullDisk(mbrSourcePath, bootSourcePath, rootSourcePath, destPath string) error {
	const MB = 1024 * 1024

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
		log.Fatalln(err)
	}

	if err := f.Truncate(int64(targetStorageBytes)); err != nil {
		log.Fatalln(err)
	}

	if err := p.Partition(f, uint64(targetStorageBytes)); err != nil {
		log.Fatalln(err)
	}

	if _, err := f.Seek(8192*512, io.SeekStart); err != nil {
		log.Fatalln(err)
	}

	bootFile, err := os.Open(bootSourcePath)
	if err != nil {
		log.Fatalln(err)
	}

	if _, err := io.Copy(f, bootFile); err != nil {
		log.Fatalln(err)
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		log.Fatalln(err)
	}

	mbrFile, err := os.Open(mbrSourcePath)
	if err != nil {
		log.Fatalln(err)
	}

	if _, err := io.Copy(f, mbrFile); err != nil {
		log.Fatalln(err)
	}

	if _, err := f.Seek(8192*512+100*MB, io.SeekStart); err != nil {
		log.Fatalln(err)
	}

	tmp, err := ioutil.TempFile("", "gokr-packer")
	if err != nil {
		log.Fatalln(err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	rootFile, err := os.Open(rootSourcePath)
	if err != nil {
		log.Fatalln(err)
	}

	if _, err := io.Copy(f, rootFile); err != nil {
		log.Fatalln(err)
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		log.Fatalln(err)
	}

	if _, err := io.Copy(f, tmp); err != nil {
		log.Fatalln(err)
	}

	return nil
}
