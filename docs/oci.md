## Constructing OCI artifacts compatible with gom
To build an OCI artifact to be compatible with the gom play feature
that can load a gokrazy image from an OCI artifacts, users will need to build an OCI artifact like so:

```
	// The OCI artifact blueprint:
	//   +---------------------------------------------------+
	//   |                                                   |
	//   |                                +----------------+ |
	//   |                             +-->      ....      | |
	//   |            +-------------+  |  +---+ Config +---+ |
	//   | (reference)+-->   ...     +--+                  | |
	//   |            ++ Manifest  ++  |  +----------------+ |
	//   |                             +-->      ...       | |
	//   |                                +---+ Layer  +---+ |
	//   |                                                   |
	//   +------------------+ registry +---------------------+
```

The OCI artifact should consist of A Manifest, a Config, and several Layers.
For it to be compatible with gom, it must:

- Manifest:
  - The Manifest must be of MediaType: `application/vnd.oci.image.manifest.v1+json`
  - Schema version 2
  - Have an annotation with key `org.opencontainers.image.created` and value a string representing the creation time in RFC3339 format.

- Config:
  - The Config must be of MediaType: `application/vnd.unknown.config.v1+json` (nested in the Manifest under, 'config')
  - Have an empty content

- Layers:
  - Have exactly 3 layers (nested in the Manifest under 'layers')
  - The 3 Layers must be of MediaType: `application/vnd.oci.image.layer.v1.tar`
  - Each of these Layers must have an annotation key `org.opencontainers.image.title` set to the filename of the specific thing they contain: "mbr.img","boot.img","root.img" (don't change them)
  - The Layer should be pushed as is and not archived nor compressed

The reference implementation on how to archive this in Go can be found here at [damdo/oci-artifacts](https://github.com/damdo/oci-artifacts).
