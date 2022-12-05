package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/damdo/gokrazy-machine/internal/disk"
	"github.com/damdo/gokrazy-machine/internal/oci"
	"github.com/damdo/gokrazy-machine/internal/ports"
	"github.com/damdo/gokrazy-machine/internal/qemu"
	"github.com/spf13/cobra"
)

// playCmd is gom play.
var playCmd = &cobra.Command{
	Use:   "play",
	Short: "starts a gokrazy machine",
	Long:  `starts a gokrazy machine`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return playImpl.play(cmd.Context(), args, cmd.OutOrStdout(), cmd.OutOrStderr())
	},
}

type playImplConfig struct {
	baseCmd     string
	arch        string
	full        string
	netNat      string
	netShared   string
	oci         string
	boot        string
	root        string
	mbr         string
	mem         string
	cores       string
	ociUser     string
	ociPassword string
}

const arm64, amd64 = "arm64", "amd64"
const modeOCI, modeFull, modeParts = "oci", "full", "parts"

var playImpl playImplConfig
var errUnsupportedArch = errors.New("error unsupported architecture")

func init() {
	playCmd.Flags().StringVar(&playImpl.arch, "arch", amd64, "arch")
	playCmd.Flags().StringVar(&playImpl.full, "full", "", "path to the img of the drive file")
	playCmd.Flags().StringVar(&playImpl.oci, "oci", "", "path to the remote oci artifact reference "+
		"(e.g. docker.io/damdo/prova:g3)")
	playCmd.Flags().StringVar(&playImpl.boot, "boot", "", "path to the boot part of the drive")
	playCmd.Flags().StringVar(&playImpl.root, "root", "", "path to the root part of the drive")
	playCmd.Flags().StringVar(&playImpl.ociUser, "oci.user", "", "the username for the OCI registry")
	playCmd.Flags().StringVar(&playImpl.ociPassword, "oci.password", "", "the password for the OCI registry")
	playCmd.Flags().StringVar(&playImpl.mbr, "mbr", "", "path to the mbr part of the drive")
	playCmd.Flags().StringVar(&playImpl.mem, "memory", "1G", "memory, expects a non-negative number below 2^64."+
		" Optional suffix k, M, G, T, P or E means kilo-, mega-, giga-, tera-, peta- and exabytes, respectively.")
	playCmd.Flags().StringVar(&playImpl.cores, "cores", "1", "number of cores available to the guest OS.")
	playCmd.Flags().StringVar(&playImpl.netNat, "net-nat", "", "net nat")
	playCmd.Flags().StringVar(&playImpl.netShared, "net-shared", "", "net shared")
}

func (r *playImplConfig) play(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	// Setup a random source.
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Setup a base temporary directory for gom.
	baseDir, err := os.MkdirTemp("", "gom")
	if err != nil {
		log.Fatalln(fmt.Errorf("error creating temporary directory: %w", err))
	}

	// These are hardcoded values for filenames
	// that we expect to find in as oci artifacts at the oci reference url
	// passed in.
	mbrSource := "mbr.img"
	bootSource := "boot.img"
	rootSource := "root.squashfs"
	destPath := "disk.img"

	diskFile, _, err := obtainDiskFile(ctx, baseDir, mbrSource, bootSource, rootSource, destPath)
	if err != nil {
		log.Fatalln(fmt.Errorf("error obtaining disk file: %w", err))
	}

	qemuArgs := []string{
		"-name", fmt.Sprintf("gokrazy-machine-%s", fmt.Sprintf("%x", rnd.Uint64())[:7]),
		"-nographic",
		"-usb",
		"-m", playImpl.mem,
		"-smp", fmt.Sprintf("cores=%s", playImpl.cores),
		"-boot", "order=d",
		"-drive", "file=" + diskFile + ",format=raw",
	}

	if err := setArchSpecificArgs(baseDir, &qemuArgs); err != nil {
		log.Fatalln(fmt.Errorf("error setting architecture specific args: %w", err))
	}

	// Check if the qemu binary set in baseCmd is present on the system.
	if _, err := exec.LookPath(playImpl.baseCmd); err != nil {
		log.Fatalln(fmt.Errorf("error while looking for qemu executable %s, is qemu installed?: %w", playImpl.baseCmd, err))
	}

	needsSudo, err := setNetworkingArgs(&qemuArgs)
	if err != nil {
		log.Fatalln(fmt.Errorf("error setting networking args: %w", err))
	}

	if needsSudo {
		// If it needs sudo, swap the arguments
		// to put "sudo" as the first "base" command.
		qemuArgs = append([]string{playImpl.baseCmd}, qemuArgs...)
		playImpl.baseCmd = "sudo"
	}

	qemuRun := exec.CommandContext(ctx, playImpl.baseCmd, qemuArgs...)

	// Pipe Stderr and Stdout to the OSes ones.
	qemuRun.Stderr = os.Stderr
	qemuRun.Stdout = os.Stdout

	log.Println("about to start qemu with config:")
	fmt.Println(fmtQemuConfig(qemuRun.Args))

	log.Println("starting qemu:")
	go func() {
		if err := qemuRun.Start(); err != nil {
			log.Fatalln(fmt.Errorf("%v: %w", qemuRun.Args, err))
		}
	}()

	// Block until a SIGINT/SIGTEM signal happens (e.g. CTRL+C).
	<-ctx.Done()

	// Cleanup the various temp/generated files used.
	if err := os.RemoveAll(baseDir); err != nil {
		log.Println(fmt.Errorf("error cleaning up temporary directory: %w", err))
	}

	// If a signal was sent, unblock the main thread and
	// kill the qemu process.
	if err := qemuRun.Process.Signal(os.Interrupt); err != nil {
		log.Print(fmt.Errorf("failed to terminate the qemu process cleanly: %w", err))

		if err := qemuRun.Process.Kill(); err != nil {
			log.Fatal(fmt.Errorf("failed to kill the qemu process,"+
				"a qemu process might have been leaked"+
				"you can try and kill it manually: %w", err))
		}
	}

	log.Println("qemu exited cleanly, shutting down")

	return nil
}

func fmtQemuConfig(cfg []string) string {
	delimiter := "-----\n"

	out := delimiter
	for _, arg := range cfg {
		if arg[:1] == "-" {
			out += fmt.Sprintf("%s ", arg)
		} else {
			out += fmt.Sprintf("%s\n", arg)
		}
	}
	out += delimiter

	return out
}

func obtainDiskFile(ctx context.Context, baseDir, mbrSourceName,
	bootSourceName, rootSourceName, destName string) (string, string, error) {
	var diskFile, mode string

	mbrSourcePath := path.Join(baseDir, mbrSourceName)
	bootSourcePath := path.Join(baseDir, bootSourceName)
	rootSourcePath := path.Join(baseDir, rootSourceName)
	destPath := path.Join(baseDir, destName)

	switch {
	case playImpl.oci != "":
		log.Println("starting in oci mode")

		// Pull OCI artifacts.
		if err := oci.Pull(ctx, playImpl.oci, playImpl.ociUser, playImpl.ociPassword, baseDir); err != nil {
			return "", "", fmt.Errorf("error pulling remote oci artifacts: %w", err)
		}

		log.Printf("merging oci artifact files (disk part images: %s, %s, %s) to a single %s image",
			mbrSourcePath, bootSourcePath, rootSourcePath, destPath)

		// Create a full disk img starting from disk pieces (mbr, boot, root).
		if err := disk.PartsToFull(mbrSourcePath, bootSourcePath, rootSourcePath, destPath); err != nil {
			log.Fatalln(fmt.Errorf("unable to create full disk img from oci artifact files: %w", err))
		}

		diskFile = destPath
		mode = modeOCI

	case playImpl.boot != "" && playImpl.root != "" && playImpl.mbr != "":
		log.Println("starting in multi part disk mode")

		log.Printf("merging disk part images: %s, %s, %s to a single %s image",
			playImpl.mbr, playImpl.boot, playImpl.root, destPath)

		// Create a full disk img starting from disk pieces (mbr, boot, root).
		if err := disk.PartsToFull(playImpl.mbr, playImpl.boot, playImpl.root, destPath); err != nil {
			log.Fatalln(
				fmt.Errorf("unable to create full disk img from files (disk part images: %s, %s, %s): %w",
					playImpl.mbr, playImpl.boot, playImpl.root, err))
		}

		diskFile = destPath
		mode = modeParts

	case playImpl.full != "":
		log.Println("starting in full disk mode")

		diskFile = playImpl.full
		mode = modeFull

	default:
		log.Fatalln("unrecognized mode, please specify either: " +
			" `--oci` or `--full` or (`--mbr` + `--boot` + `--root`)")
	}

	return diskFile, mode, nil
}

func setArchSpecificArgs(baseDir string, qemuArgs *[]string) error {
	var archArgs []string
	var biosFilePerm fs.FileMode = 0644

	switch playImpl.arch {
	case amd64:
		playImpl.baseCmd = "qemu-system-x86_64"

	case arm64:
		playImpl.baseCmd = "qemu-system-aarch64"
		qemuBios := path.Join(baseDir, "QEMU_EFI.fd")

		archArgs = append(
			archArgs,
			"-machine", "virt,highmem=off",
			"-cpu", "cortex-a72",
			"-bios", qemuBios,
		)

		biosFile, err := qemu.EmbedFS.ReadFile("QEMU_EFI.fd")
		if err != nil {
			return fmt.Errorf("error reading embedded %s bios file: %w", arm64, err)
		}

		if err := os.WriteFile(qemuBios, biosFile, biosFilePerm); err != nil {
			return fmt.Errorf("error writing embedded %s bios file to disk: %w", arm64, err)
		}

	default:
		return fmt.Errorf("%w: %s", errUnsupportedArch, playImpl.arch)
	}

	*qemuArgs = append(*qemuArgs, archArgs...)
	return nil
}

func setNetworkingArgs(qemuArgs *[]string) (bool, error) {
	var needsSudo bool
	defaultOpenPortsNumber := 3

	switch {
	case playImpl.netNat == "" && playImpl.netShared == "":
		freePorts, err := ports.GetFreePorts(defaultOpenPortsNumber)
		if err != nil {
			return false, fmt.Errorf("error getting free ports: %w", err)
		}

		// NAT net with port forwarding.
		netNatDefault := []string{
			"-device", "e1000,netdev=net0",
			"-netdev", "user,id=net0" +
				",hostfwd=tcp::" + fmt.Sprint(freePorts[0]) + "-:80" +
				",hostfwd=tcp::" + fmt.Sprint(freePorts[1]) + "-:443" +
				",hostfwd=tcp::" + fmt.Sprint(freePorts[2]) + "-:22",
		}

		*qemuArgs = append(*qemuArgs, netNatDefault...)

	case playImpl.netNat != "":
		ports := strings.Split(playImpl.netNat, ",")

		netNat := []string{"-netdev", "user,id=net0", "-device", "e1000,netdev=net0"}
		for _, p := range ports {
			netNat[1] += ",hostfwd=tcp::" + p
		}

		*qemuArgs = append(*qemuArgs, netNat...)

	case playImpl.netShared != "":
		if runtime.GOOS != "darwin" {
			log.Fatalln("error --net-shared is only supported on macOS")
		}

		needsSudo = true
		addrRange := strings.Split(playImpl.netShared, ",")
		netShared := []string{"-netdev", "vmnet-shared,id=internal", "-device", "e1000,netdev=internal"}

		r := fmt.Sprintf(",start-address=%s,end-address=%s,subnet-mask=%s",
			addrRange[0], addrRange[1], addrRange[2])

		netShared[1] += r

		*qemuArgs = append(*qemuArgs, netShared...)
	}

	return needsSudo, nil
}
