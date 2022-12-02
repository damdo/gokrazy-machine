## gom (gokrazy-machine)

A lightweight virtual/emulated machine to run and develop for [gokrazy](gokrazy.org).

It supports all major Linux distributions and macOS (both intel and apple silicon (M1,M2,..)) >= `12.x`.

### pre-install requirements

A hard requirement for this to work is to install `qemu` >= `7.1.0`.

On macOS this can be done directly with:
```
brew install qemu
```

On Linux you might need to compile and install qemu with:
```
curl -SLO https://download.qemu.org/qemu-7.1.0.tar.xz
tar -xf qemu-7.1.0.tar.xz
cd qemu-7.1.0
./configure
make -j $(nproc)
sudo make install
```

### install
```
go install github.com/damdo/gokrazy-machine/cmd/gom
```

Check version
```
gom version
```

### play

The main command is
```
gom play --full /tmp/drive.img
```

but there are various modes with which you can run gom.

### with various disk images

Run machine from a **full disk img**.
```
gom play --full /tmp/drive.img
```

Run machine from **different disk parts (boot,root,mbr)**.
```
gom play --boot=/tmp/boot.fat --root=/tmp/root.squashfs --mbr=/tmp/mbr.img
```

Run machine from **remote OCI artifact** (the image won't be kept locally).
```
gom play --arch amd64 --oci docker.io/damdo/gokrazy:sample-amd64

# or with an arm64 image
gom play --arch arm64 --oci docker.io/damdo/gokrazy:sample-arm64
```

### with various networking setups

By default a gom machine will use a nat network, and will map port 80, 443 and 22 to random ports.
These random assigned ports can be found in the log output of gom, at the top, before the machine is started. 

But if you need to do specific or extra mappings, or use different modes, here is how you can do it.

Run gom machine in **NAT network**, with specific port forwarding.
Set `--net-nat="<outer-port>-:<inner-port>,<outer-port>-:<inner-port>"`
```
gom play ... --net-nat="8181-:80,2222-:22"
```

[Supported only for macOS]
Run gom machine in **shared network**, with specific IP range.
This can be set with --net-shared, a comma separated string
where users can set `--net-shared="<start-ip,end-ip,subnet-mask>"`
```
gom play ... --net-shared="192.168.70.1,192.168.70.254,255.255.255.0"
```

### with various target architectures
By default gom will use the `amd64`/`x86_64` architecture as the target machine architecture.
But `arm64` can also be set.

For `amd64`/`x86_64`:
```
gom play --arch="amd64"
```

For `arm64`:
```
gom play --arch="arm64"
```

### with custom memory for the guest VM
By default gom will use `1G` of memory for the guest VM.
It can be customized with
```
gom play --memory="2G"
```
