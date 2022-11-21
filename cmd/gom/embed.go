package main

import "embed"

//go:embed bin/QEMU_EFI.fd
var embedFS embed.FS
