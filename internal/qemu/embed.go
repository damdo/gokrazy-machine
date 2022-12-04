package qemu

import "embed"

//go:embed QEMU_EFI.fd
var EmbedFS embed.FS
