## DOCKER CP DESIGN DOC

### Initial Rundown of docker cp

Docker cp has many possible scenarios that we will have to account for and they will be described in this section.

We will be supporting docker cp in it's entirety. This means we will need to copy files to and from a container regardless of whether it is on or off. This will lead to 4 scenarios. 

1. The container is *on*, and we want to copy data *from* it.

2. The container is *on*, and we want to copy data *to* it.

3. The container is *off*, and we want to copy data *from* it.(will constitute multiple calls from the personality)

4. The container is *off*, an we want to copy data *to* it.(will constitute multiple calls from the personality)

According to the docker remote api the target for a copy must be in `identity (no compression), gzip, bzip2, xz`. As the docker cli handles packaging a *to* request we can expect on of these formats. We must package our *from* response in one of these formats as well. 

 _NOTE_: docker has discussed copying between containers in the past. This will be yet another set of 4 possibilities and endpoints. Something that we can add to this plan later if it is needed in the future. 

Here is a more complex look at the call operation/state flow.

|Operation| state| volumes | scenario |
|---|---|---|---|
|  Export | ContainerRunning | No | Single call from personality to portlayer. Guest Tools will be used on the target rather than `Read` |
| Export | ContainerRunning | Yes |  Single call from personality to portlayer. Guest Tools will be used on the target rather than `Read` |
| Export | ContainerStopped | No | Single call from personality to portlayer. `Read` is called based on the supplied filespec. |
| Export | ContainerStopped | Yes | Multiple calls from personality to portlayer. One for the r/w layer, and n more calls where n is the number of volumes. `Read` is invoked n+1 times based on each call. Powerstatus likely won't matter here because we mount non-persistent disks|
| Import |  ContainerRunning | No | Single call from personality to portlayer. Guest tools will be used on the target rather than a `Write` |
|  Import | ContainerRunning | Yes |  Single call from personality to portlayer. Guest tools will be used on the target rather than a `Write` |
| Import | ContainerStopped |  No | Single call from personality to portlayer. `Write` will be used to mount the r/w layer and then write the contents based on the supplied filespec  |
| Import | ContainerStopped | Yes |  Multiple calls from personality to portlayer. One for the r/w layer, and n more calls where n is the number of volumes. `Write` is invoked n+1 times based on each call from the personality. If the container is started during this time we cannot mount the volumes or the r/w layer and we will report a failure requesting the user to try again. Alternatively, we block start events until operation completion. |


### Personality to Portlayer Communication

Since all 4 of these must be supported we will require both distinctions for when the container is off and which endpoint to call to distinguish between *to* and *from* operations. 

The proposed solution is to have two endpoints designed for the portlayer api swagger server. Both endpoints will exist along the same request path as two different verbs. The request path should be as such : 

The portlayer functional target for `ExportArchive` and `ImportArchive` will need to be called multiple times from the personality for each device that constitutes the full filesystem of the container. A `PathSpec` struct will be used to mark inclusion and exclusion paths. We will also need to know when to strip the paths provided to the portlayer since the view of a volume would be `/` while in a running container the target path could be `/path/to/volume/` where the path before the final `/` exists on the container r/w layer(or another volume, but we are not worrying about that now.) Because there will be multiple calls for the potential Import of the archive(write calls) we will need to have a way to pass or reference the stream of information to the portlayer multiple times. This may constitute another input for the function header in order to identify the stream that we care about. 

the portlayer functional targets will look as such: 

__ExportArchive__

```
// ID : container/vm target ID mainly for power state checking in the portlayer to determine Exports logical behavior.
// deviceID : target deviceID for copying
// fileSpec : just a map[string]string of keyed paths to exclusion and strip operations determined by the desired behavior of the read
// 
func Read(ID, deviceID, filespec map[string]string) (io.reader, error)

// no need for the data bool since we know that this operation involves data. we can set that as always true on the portlayer side of things.
```


__ImportArchive__
```
// StoreID : ID of the store where the target device resides be it image store, volume store, or something else in the future.
// ID : target container/vm for the portlayer to check powerstate to determine the logical approach of the ImportArchive function
// deviceID : device id found in the targeted store that is to receive the archive
// fileSpec : just a map[string]string of keyed paths to exclusion and strip operations determined by the desired behavior of the write
// tarStream : the actual tar stream to be imported to the target
func Write(ID, deviceID string, fileSpec map[string]string, tarStream io.Writer) error
```

Note: this call will involve multiple docker endpoints, Stat is needed with we plan to support the `-L` functionality. We will also need the interaction piece for streaming tar archive's to the portlayer with the same tar stream from docker being streamed to multiple calls(hopefully we might have a way to not stream the same data several times over the network).

## Personality design


### Copy Operation

The personality has Three endpoints that are called depending on the to/from situation above(5 if you count the two deprecated calls that were made previously.). The portlayer will divide the behavior of to/from as two separate endpoints. These endpoints will behave in an `Export/Import` behavior. Aside from the Import/Export behavior the docker cli also calls `ContainerStatPath` when determining whether to follow symlinks via the `-L` option. We will need this endpoint implemented all the way through to the portlayer if we want to support following symlinks.

Error codes expected by docker : 

```
200 

400

403 - in the event of a readonly filesystem

404

409

500

```

Docker does not do much in the way of string checking as far as the path is concerned. This is likely not a big concern for the personality as the actual copy command will fail if a bad path string is provided. 

For the multiple calls to the portlayer we will need to assemble Pathspecs which detail inclusion, exclusion, rebase,and strip configurations for the tar stream on both reads and writes. Below will be some scenarios and what the path spec should look like for those scenarios.

Exclude the tether path. Exclude any paths that docker does not allow for a copy. the filespec map with a serialized struct or keys for `exclusion`, `strip`.

examples of a pathspec(please note that these are simple scenarios for the time being):
```
For a CopyTo to path "/volumes" :
container has one volume mounted at /volumes/data

this will invoke two write calls. 

first pathspec: 

// this pathspec would be empty since you would be writing against the r/w of the containerFS and the starting path would be '/' of the containerFS. No stripping of the write headers necessary.
spec.rebase="volumes"
spec.strip = ""
spec.includes = map[string]string {"": struct{}{}}
spec.excludes = map[string]string {"volumes/data": struct{}{}}

Second Pathspec:
spec.Strip = "volumes/data/"  // stripped since the volume will be mounted and the starting path will be "/"
spec.Rebase = ""
spec.includes = map[string]string {"": struct{}{}}
}
```

```
For a Copy from path "/volumes/" :
container has one volume mounted at /volumes/data

this will invoke two read calls

// this pathspec would be empty since you would be writing against the r/w of the containerFS and the starting path would be '/' of the containerFS. No rebase necessary for the headers.
spec.Rebase = "volumes"
spec.Strip = "volumes"
// in this case strip and rebase are the same since this is a 1st level directory e.g for a target of /volumes/data rebase would be "data" and strip would be "volumes/data" 
spec.include = map[string]string {"volumes":struct{}{}}
spec.Exclude = map[string]string {"": struct{}{}}

// this pathspec will be for the volume filesystem
spec.Rebase = "volumes"
spec.Strip = ""
spec.include = map[string]string {"":struct{}{}}

```


NOTE: use TarAppender in docker/pkg/archive to possibly merge the different tar streams during an ExportArchive. 

### Stat Operation



## Portlayer Design



 There will be two situations to be concerned with in the portlayer. When the container is *on* and when the container is *off*.
 
 in the event of the *off* scenario we will convey the requested operations to the `Read`/`Write` calls in the storage portion of the portlayer. The results of those calls should be returned to the user.
 
 Regardless of whether the container is on or off. Some investigation has shown that it works in the scenario of both containers being on, off, and one on and one off. We will need to architect a way for this to work for us. If the target is a volume that is attached to a container we will need to understand what is needed in the case of a vmdk volume target. This behavior should not be an issue for the nfs volume store targets.
 
 The portlayer should behave at a basic level as such: 
 
 ```
 1. look up appropriate storage object
2. check object usage
3. mount object or initiate copy based on (2)
3b. check for error in initialization of copy due to power state change, repeat (2) if found
4. mounted disk prevents incompatible operations, or, tether copy in progress squashes in guest shutdown
 ```
 
 some additiona notes surrounding the portlayer and swagger endpoint design:
 

use query string to pass FilterSpec so that it's not needed to be packaged in a custom format in body along with the tar
this assumes that callers have some knowledge of mount topology and correctly populate the FilterSpec to avoid recursing into mounts that will be directly addressed by a later call.
because it's possible for the various storage elements of a container to be in different states (e.g. container is not running so r/w and volumeA are not in use, but volumeB from the container is now used by container X) we require that separate calls be made to Import/Export for each of the distinct storage elements. This allows those calls to be routed appropriately based on current owners and usage of the element in question.
 
 
### Stat Operation

We will need a `stat` endpoint which will be used to target devices with a filesystem that can return fileinfo back to the caller. This will need to be tolerant of the power status of the target, if the target container/vm is on then we can use guest tools. Otherwise, mounting the appropriate device for the stat is necessary. Like the Read/Write calls we should expect a store ID and a device ID in addition to the target of the stat operation. if the compute is not active then we should mount the target device specified for the stat. 

```
underlying filesystem stat :
// storename : this is the store that the target device resides on 
// deviceID : this is the target device for the stat operation
// target : is the filesys
//
// stat will mount the target and stat the filesystem target in order to obtain the requested filesystem info.
func Stat(storename string, deviceID string, spec archive.FilterSpec) (FileInfo, error)

// it is the responsibility of the caller to determine the status of this device. If it is already mounted or in use the caller must determine the action to be taken. 
```

 
### Portlayer Import/Export Behavior for writing and reading to storage devices.
The following are the core interfaces that allow hiding of the current usage state of a given storage element such as a volume:

```
When a container is online and a copy is attempted guest tools will be utilized to move the files onto the container. Below are the defined interfaces that will be used for reading and writing to the devices. Note that there can be multiple datasources and datasinks for the same device backing. This is due to online and offline behavior. 

A docker stop should be blocked until the copy.(currently this is not the case) 

// DataSource defines the methods for exporting data from a specific storage element as a tar stream
type DataSource interface {
	// Close releases all resources associated with this source. Shared resources should be reference counted.
	io.Closer

	// Export performs an export of the specified files, returning the data as a tar stream. This is single use; once
	// the export has completed it should not be assumed that the source remains functional.
	//
	// spec: specifies which files will be included/excluded in the export and allows for path rebasing/stripping
	// data: if true the actual file data is included, if false only the file headers are present
	Export(op trace.Operation, spec *archive.FilterSpec, data bool) (io.ReadCloser, error)

	// Source returns the mechanism by which the data source is accessed
	// Examples:
	//     vmdk mounted locally: *os.File
	//     nfs volume:  		 XDR-client
	//     via guesttools:  	 toolbox client
	Source() interface{}
}

// DataSink defines the methods for importing data to a specific storage element from a tar stream
type DataSink interface {
	// Close releases all resources associated with this sink. Shared resources should be reference counted.
	io.Closer

	// Import performs an import of the tar stream to the source held by this DataSink.  This is single use; once
	// the export has completed it should not be assumed that the sink remains functional.
	//
	// spec: specifies which files will be included/excluded in the import and allows for path rebasing/stripping
	// tarStream: the tar stream to from which to import data
	Import(op trace.Operation, spec *archive.FilterSpec, tarStream io.ReadCloser) error

	// Sink returns the mechanism by which the data sink is accessed
	// Examples:
	//     vmdk mounted locally: *os.File
	//     nfs volume:  		 XDR-client
	//     via guesttools:  	 toolbox client
	Sink() interface{}
}

// Importer defines the methods needed to write data into a storage element. This should be implemented by the various
// store types.
type Importer interface {
	// Import allows direct construction and invocation of a data sink for the specified ID.
	Import(op trace.Operation, id string, spec *archive.FilterSpec, tarStream io.ReadCloser) error

	// NewDataSink constructs a data sink for the specified ID within the context of the Importer. This is a single
	// use sink which may hold resources until Closed.
	NewDataSink(op trace.Operation, id string) (DataSink, error)
}

// Exporter defines the methods needed to read data from a storage element, optionally diff with an ancestor. This
// shoiuld be implemented by the various store types.
type Exporter interface {
	// Export allows direct construction and invocation of a data source for the specified ID.
	Export(op trace.Operation, id, ancestor string, spec *archive.FilterSpec, data bool) (io.ReadCloser, error)

	// NewDataSource constructs a data source for the specified ID within the context of the Exporter. This is a single
	// use source which may hold resources until Closed.
	NewDataSource(op trace.Operation, id string) (DataSource, error)
}
```

This provides a pair of helper functions per store, supporting generalized implementation of the above Import/Export logic:

```
// Resolver defines methods for mapping ids to URLS, and urls to owners of that device
type Resolver interface {
	// URL returns a url to the data source representing `id`
	// For historic reasons this is not the same URL that other parts of the storage component use, but an actual
	// URL suited for locating the storage element without having additional precursor knowledge.
        // Of the form `ds://[datastore]/path/on/datastore/element.vmdk`
	URL(op trace.Operation, id string) (*url.URL, error)

	// Owners returns a list of VMs that are using the resource specified by `url`
	Owners(op trace.Operation, url *url.URL, filter func(vm *mo.VirtualMachine) bool) ([]*vm.VirtualMachine, error)
}
```

Example usage:

```
func (h *handler) ImportArchive(store, id string, spec *archive.FilterSpec, tar io.ReadCloser) middleware.Responder {
	op := trace.NewOperation(context.Background(), "ImportArchive: %s:%s", store, id)

	s, ok := storage.GetImporter(store)
	if !ok {
		op.Errorf("Failed to locate import capable store %s", store)
		op.Debugf("Available importers are: %+q", storage.GetImporters())

		return storage.NewImportArchiveNotFound()
	}

	err := s.Import(op, id, spec, tar)
	if err != nil {
                // This should be usefully typed errors
		return storage.NewExportArchiveInternalServerError()
	}

	return storage.NewImportArchiveOK()
}
```
