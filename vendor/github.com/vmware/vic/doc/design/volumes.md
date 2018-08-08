# Volume support

Volumes are mutable disk devices, backed by `.vmdk`, whos lifetime is decoupled from that of the container VM.

### Requirements
- To create a `volume` by `ID` in PL on the specified datastore + path via `volumestore` name and return a URI pointing it.
- To create "anonymous" `volumes and identify them with a generated `ID`.
- To (statefully) remove a `volume` via the PL by its URI.
- To reference an existing `volume` in the `spec` for a new container by its vmkd path
- Store (immutable) metadata in `"key"/[]byte{"value"}` form.


### Filesystem requirements
Mirrors what exists in the storage layer for images.  Currently there is an implementation for `ext4`, but the underlying `disk` implementation is opaque to this.  In short, we'll use `ext4` for now, but using any other filesystem shouldn't be a huge change.


### Datastore destination
The VI admin will specify datastore + paths which volumes should be created on, giving each of them a `volumestore` to identify them.  For instance `[datastore1] /path/to/volumes` can be identified by volumestore name `datastoreWithVolumes`.  The the docker user can supply this volumestore name to as a driver arg when creating the volume.  The result will be a `.vmdk` created in the `[datastore1] /path/to/volumes/VIC/<vch uuid>/volumes/<volume name>/<volume name>.vmdk`

### Definitions
```
// Unique identifier for the volume.  Enumeration of volumes is done via these tags.
type Tag string

type Volume struct {
	// Identifies the volume
	Tag 	Tag

	// The datastore the volume lives on
	Datastore *object.Datastore

	// Metadata the volume is included with.  Is persisted along side the volume vmdk.
	Info    map[string][]byte

	// Namespace in the storage layer to look up this volume.
	SelfLink url.URL
}

// VolumeStorer is an interface to create, remove, enumerate, and get Volumes.
type VolumeStorer interface {
	// Creates a volume on the given volume store, of the given size, with the given metadata.
	VolumeCreate(ctx context.Context, ID string, store *url.URL, capacityKB uint64, info map[string][]byte) (*Volume, error)

	// Get an existing volume via it's ID.
	VolumeGet(ctx context.Context, ID string) (*Volume, error)

	// Destroys a volume
	VolumeDestroy(ctx context.Context, ID string) error

	// Lists all volumes
	VolumesList(ctx context.Context) ([]*Volume, error)
}
```
