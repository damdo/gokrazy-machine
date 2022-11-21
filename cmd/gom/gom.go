package main

import (
	"flag"
	"math/rand"
	"os/exec"
	"os/signal"
	"path"
	"runtime"
	"strings"
	"syscall"
	"time"

	"context"
	"fmt"
	"log"
	"os"
)

const arm64, amd64 = "arm64", "amd64"
const modeOCI, modeFull, modeParts = "oci", "full", "parts"

var baseCmd, arch, full, netNat, netShared, oci, boot, root, mbr, mem, cores string
var needsSudo bool

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	flag.StringVar(&arch, "arch", amd64, "arch")
	flag.StringVar(&full, "full", "", "path to the img of the drive file")
	flag.StringVar(&oci, "oci", "", "path to the remote oci artifact reference (e.g. docker.io/damdo/prova:g3)")
	flag.StringVar(&boot, "boot", "", "path to the boot part of the drive")
	flag.StringVar(&root, "root", "", "path to the root part of the drive")
	flag.StringVar(&mbr, "mbr", "", "path to the mbr part of the drive")
	flag.StringVar(&mem, "memory", "1G", "memory, expects a non-negative number below 2^64."+
		" Optional suffix k, M, G, T, P or E means kilo-, mega-, giga-, tera-, peta- and exabytes, respectively.")
	flag.StringVar(&cores, "cores", "1", "number of cores available to the guest OS.")
	flag.StringVar(&netNat, "net-nat", "", "net nat")
	flag.StringVar(&netShared, "net-shared", "", "net shared")

	flag.Parse()

	if err := validateFlags(); err != nil {
		log.Fatalln(fmt.Errorf("invalid flags: %w", err))
	}

	// Setup a random source.
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	baseDir, err := os.MkdirTemp("", "gom")
	if err != nil {
		log.Fatalln(fmt.Errorf("error creating temporary directory: %w", err))
	}

	// These are hardcoded values for filenames
	// that we expect to find in as oci artifacts at the oci reference url
	// passed in.
	var mbrSource string = "mbr.img"
	var bootSource string = "boot.img"
	var rootSource string = "root.squashfs"
	var destPath string = "disk.img"

	diskFile, _, err := obtainDiskFile(ctx, baseDir, mbrSource, bootSource, rootSource, destPath)
	if err != nil {
		log.Fatalln(fmt.Errorf("error obtaining disk file: %w", err))
	}

	qemuArgs := []string{
		"-name", fmt.Sprintf("gokrazy-machine-%s", fmt.Sprintf("%x", rnd.Uint64())[:7]),
		"-nographic",
		"-usb",
		"-m", mem,
		"-smp", fmt.Sprintf("cores=%s", cores),
		"-boot", "order=d",
		"-drive", "file=" + diskFile + ",format=raw",
	}

	if err := setArchSpecificArgs(baseDir, &qemuArgs); err != nil {
		log.Fatalln(fmt.Errorf("error setting architecture specific args: %w", err))
	}

	// Check if the qemu binary set in baseCmd is present on the system.
	if _, err := exec.LookPath(baseCmd); err != nil {
		log.Fatalln(fmt.Errorf("error while looking for qemu executable %s, is qemu installed?: %w", baseCmd, err))
	}

	needsSudo, err := setNetworkingArgs(&qemuArgs)
	if err != nil {
		log.Fatalln(fmt.Errorf("error setting networking args: %w", err))
	}

	if needsSudo {
		// If it needs sudo, swap the arguments
		// to put "sudo" as the first "base" command.
		qemuArgs = append([]string{baseCmd}, qemuArgs...)
		baseCmd = "sudo"
	}

	qemu := exec.CommandContext(ctx, baseCmd, qemuArgs...)
	// Pipe Stderr and Stdout to the OSes ones.
	qemu.Stderr = os.Stderr
	qemu.Stdout = os.Stdout

	log.Println("about to start qemu with config:")
	fmt.Println(fmtQemuConfig(qemu.Args))

	log.Println("starting qemu:")
	go func() {
		if err := qemu.Start(); err != nil {
			log.Fatalln(fmt.Errorf("%v: %v", qemu.Args, err))
		}
	}()

	// Block until a SIGINT/SIGTEM signal happens (e.g. CTRL+C).
	<-ctx.Done()

	// Cleanup the various temp/generated files used.
	os.RemoveAll(baseDir)

	// If a signal was sent, unblock the main thread and
	// kill the qemu process.
	if err := qemu.Process.Signal(os.Interrupt); err != nil {
		log.Print(fmt.Errorf("failed to terminate the qemu process cleanly: %w", err))

		if err := qemu.Process.Kill(); err != nil {
			log.Fatal(fmt.Errorf("failed to kill the qemu process,"+
				"a qemu process might have been leaked"+
				"you can try and kill it manually: %w", err))
		}
	}

	log.Println("qemu exited cleanly, shutting down")
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

func obtainDiskFile(ctx context.Context, baseDir, mbrSourceName, bootSourceName, rootSourceName, destName string) (string, string, error) {
	var diskFile, mode string

	mbrSourcePath := path.Join(baseDir, mbrSourceName)
	bootSourcePath := path.Join(baseDir, bootSourceName)
	rootSourcePath := path.Join(baseDir, rootSourceName)
	destPath := path.Join(baseDir, destName)

	switch {
	case oci != "":
		log.Println("starting in oci mode")

		// Pull OCI artifacts.
		if err := ociPull(ctx, oci, "", "", baseDir); err != nil {
			return "", "", fmt.Errorf("error pulling remote oci artifacts: %w", err)
		}

		log.Printf("merging oci artifact files (disk part images: %s, %s, %s) to a single %s image",
			mbrSourcePath, bootSourcePath, rootSourcePath, destPath)

		// Create a full disk img starting from disk pieces (mbr, boot, root).
		if err := createFullDisk(mbrSourcePath, bootSourcePath, rootSourcePath, destPath); err != nil {
			log.Fatalln(fmt.Errorf("unable to create full disk img from oci artifact files: %w", err))
		}

		diskFile = destPath
		mode = modeOCI

	case boot != "" && root != "" && mbr != "":
		log.Println("starting in multi part disk mode")

		log.Printf("merging disk part images: %s, %s, %s to a single %s image", mbr, boot, root, destPath)

		// Create a full disk img starting from disk pieces (mbr, boot, root).
		if err := createFullDisk(mbr, boot, root, destPath); err != nil {
			log.Fatalln(
				fmt.Errorf("unable to create full disk img from files (disk part images: %s, %s, %s): %w",
					mbr, boot, root, err))
		}

		diskFile = destPath
		mode = modeParts

	case full != "":
		log.Println("starting in full disk mode")

		diskFile = full
		mode = modeFull

	default:
		log.Fatalln("unrecognized mode, please specify either: `--oci` or `--full` or (`--mbr` + `--boot` + `--root`)")
	}

	return diskFile, mode, nil
}

func setArchSpecificArgs(baseDir string, qemuArgs *[]string) error {
	var archArgs []string

	switch arch {
	case amd64:
		baseCmd = "qemu-system-x86_64"

	case arm64:
		baseCmd = "qemu-system-aarch64"
		qemuBios := path.Join(baseDir, "QEMU_EFI.fd")

		archArgs = append(
			archArgs,
			"-machine", "virt,highmem=off",
			"-cpu", "cortex-a72",
			"-bios", qemuBios,
		)

		biosFile, err := embedFS.ReadFile("bin/QEMU_EFI.fd")
		if err != nil {
			log.Fatalln(fmt.Errorf("error reading embedded %s bios file: %w", arm64, err))
		}

		if err := os.WriteFile(qemuBios, biosFile, 777); err != nil {
			log.Fatalln(fmt.Errorf("error writing embedded %s bios file to disk: %w", arm64, err))
		}

	default:
		log.Fatalf("error: unsupported architecture '%s'\n", arch)
	}

	*qemuArgs = append(*qemuArgs, archArgs...)
	return nil
}

func setNetworkingArgs(qemuArgs *[]string) (bool, error) {
	var needsSudo bool
	switch {
	case netNat == "" && netShared == "":
		freePorts, err := getFreePorts(3)
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

	case netNat != "":
		ports := strings.Split(netNat, ",")

		netNat := []string{"-netdev", "user,id=net0", "-device", "e1000,netdev=net0"}
		for _, p := range ports {
			netNat[1] += ",hostfwd=tcp::" + p
		}

		*qemuArgs = append(*qemuArgs, netNat...)

	case netShared != "":
		if runtime.GOOS != "darwin" {
			log.Fatalln("error --net-shared is only supported on macOS")
		}

		needsSudo = true
		addrRange := strings.Split(netShared, ",")
		netShared := []string{"-netdev", "vmnet-shared,id=internal", "-device", "e1000,netdev=internal"}

		r := fmt.Sprintf(",start-address=%s,end-address=%s,subnet-mask=%s", addrRange[0], addrRange[1], addrRange[2])
		netShared[1] += r

		*qemuArgs = append(*qemuArgs, netShared...)
	}

	return needsSudo, nil
}

func cleanup(ctx context.Context, arch, mode string) error {
	if arch == arm64 {
		if err := os.Remove("./.QEMU_EFI.fd"); err != nil {
			return fmt.Errorf("error cleaning up temporary %s bios file: %w", arm64, err)
		}
	}

	// TODO, remove disk img files, if it was downloaded/converted.

	return nil
}
