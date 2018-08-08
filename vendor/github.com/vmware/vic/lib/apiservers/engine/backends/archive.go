// Copyright 2017 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package backends

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/tchap/go-patricia/patricia"

	"github.com/vmware/vic/lib/apiservers/engine/backends/cache"
	viccontainer "github.com/vmware/vic/lib/apiservers/engine/backends/container"
	"github.com/vmware/vic/lib/apiservers/engine/errors"
	"github.com/vmware/vic/lib/apiservers/engine/proxy"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/storage"
	vicarchive "github.com/vmware/vic/lib/archive"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/pkg/trace"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/archive"
)

// ContainerArchivePath creates an archive of the filesystem resource at the
// specified path in the container identified by the given name. Returns a
// tar archive of the resource and whether it was a directory or a single file.
func (c *ContainerBackend) ContainerArchivePath(name string, path string) (io.ReadCloser, *types.ContainerPathStat, error) {
	op := trace.NewOperation(context.Background(), "")
	defer trace.End(trace.Audit(name, op))

	path = "/" + strings.TrimPrefix(path, "/")
	vc := cache.ContainerCache().GetContainer(name)
	if vc == nil {
		return nil, nil, errors.NotFoundError(name)
	}

	stat, err := c.ContainerStatPath(name, path)
	if err != nil {
		return nil, nil, err
	}

	reader, err := c.exportFromContainer(op, vc, path)
	if err != nil {
		if errors.IsResourceInUse(err) {
			err = fmt.Errorf("ContainerArchivePath failed, resource in use: %s", err.Error())
		}
		return nil, nil, errors.InternalServerError(err.Error())
	}

	return reader, stat, nil
}

func (c *ContainerBackend) exportFromContainer(op trace.Operation, vc *viccontainer.VicContainer, path string) (io.ReadCloser, error) {
	mounts := proxy.MountsFromContainer(vc)
	mounts = append(mounts, types.MountPoint{Destination: "/"})
	readerMap := NewArchiveStreamReaderMap(op, mounts, path)

	readers, err := readerMap.ReadersForSourcePath(archiveProxy, vc.ContainerID, path)
	if err != nil {
		op.Errorf("Errors getting readers for export: %s", err.Error())
		return nil, err
	}
	count := len(readers)
	op.Infof("Got %d archive readers", count)

	// We want to combine the streams, so need to strip the end-of-archive elements for all but the last
	strippersWithCloser := make([]io.Reader, len(readers))
	i := 0
	for ; i < count-1; i++ {
		stripper := vicarchive.NewStripper(op, tar.NewReader(readers[i]), readers[i].Close)
		strippersWithCloser[i] = stripper
		op.Debugf("Added stripping reader: %p", stripper)
	}
	op.Debugf("Adding closing reader: %p", readers[i])
	strippersWithCloser[i] = readers[i]

	return vicarchive.MultiReader(strippersWithCloser...), nil
}

// ContainerCopy performs a deprecated operation of archiving the resource at
// the specified path in the container identified by the given name.
func (c *ContainerBackend) ContainerCopy(name string, res string) (io.ReadCloser, error) {
	return nil, errors.APINotSupportedMsg(ProductName(), "ContainerCopy")
}

// ContainerExport writes the contents of the container to the given
// writer. An error is returned if the container cannot be found.
func (c *ContainerBackend) ContainerExport(name string, out io.Writer) error {
	return errors.APINotSupportedMsg(ProductName(), "ContainerExport")
}

// ContainerExtractToDir extracts the given archive to the specified location
// in the filesystem of the container identified by the given name. The given
// path must be of a directory in the container. If it is not, the error will
// be ErrExtractPointNotDirectory. If noOverwriteDirNonDir is true then it will
// be an error if unpacking the given content would cause an existing directory
// to be replaced with a non-directory and vice versa.
func (c *ContainerBackend) ContainerExtractToDir(name, path string, noOverwriteDirNonDir bool, content io.Reader) error {
	op := trace.NewOperation(context.Background(), "")
	defer trace.End(trace.Audit(name, op))

	path = "/" + strings.TrimPrefix(path, "/")

	vc := cache.ContainerCache().GetContainer(name)
	if vc == nil {
		return errors.NotFoundError(name)
	}

	err := c.importToContainer(op, vc, path, content)
	if err != nil && errors.IsResourceInUse(err) {
		op.Errorf("ContainerExtractToDir failed, resource in use: %s", err.Error())

		err = fmt.Errorf("Resource in use")
	}

	return err
}

func (c *ContainerBackend) importToContainer(op trace.Operation, vc *viccontainer.VicContainer, target string, content io.Reader) (err error) {
	rawReader, err := archive.DecompressStream(content)
	if err != nil {
		op.Errorf("Input tar stream to ContainerExtractToDir not recognized: %s", err.Error())
		return errors.StreamFormatNotRecognized()
	}

	tarReader := tar.NewReader(rawReader)
	mounts := proxy.MountsFromContainer(vc)
	mounts = append(mounts, types.MountPoint{Destination: "/"})
	writerMap := NewArchiveStreamWriterMap(op, mounts, target)
	defer func() {
		// This should shutdown all the stream connections to the portlayer.
		e1 := writerMap.Close(op)
		if e1 != nil {
			err = e1
			op.Debugf("import to container: assigned err as %v", err)
		}
	}()

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			op.Errorf("Error reading tar header from client archive: %s", err)
			return err
		}

		// Lookup the writer for that mount prefix
		tarWriter, err := writerMap.WriterForAsset(archiveProxy, vc.ContainerID, target, *header)
		if err != nil {
			return err
		}

		if err = tarWriter.WriteHeader(header); err != nil {
			op.Errorf("Error while copying tar header %#v: %s", *header, err.Error())
			return err
		}

		if _, err = io.Copy(tarWriter, tarReader); err != nil {
			op.Errorf("Error while copying tar data for %s: %s", header.Name, err.Error())
			return err
		}
		// TODO: change this to log level 3
		if vchConfig.Cfg.Diagnostics.DebugLevel >= 1 {
			op.Debugf("Wrote entry: %s", header.Name)
		}
	}

	return nil
}

// ContainerStatPath stats the filesystem resource at the specified path in the
// container identified by the given name.
func (c *ContainerBackend) ContainerStatPath(name string, path string) (stat *types.ContainerPathStat, err error) {
	op := trace.NewOperation(context.Background(), "")
	defer trace.End(trace.Audit(name, op))

	op.Debugf("path received by statpath %s", path)

	vc := cache.ContainerCache().GetContainer(name)
	if vc == nil {
		return nil, errors.NotFoundError(name)
	}

	// trim / and . off from path and then append / to ensure the format is correct
	path = filepath.Clean(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	mounts := proxy.MountsFromContainer(vc)
	mounts = append(mounts, types.MountPoint{Destination: "/"})

	// handle the special case of targeting a volume mount point before it exists.
	// this will be important for non started container cp, will also be important
	// to certain behaviors for diff on a non started container.
	if stat, succeed := tryFakeStatPath(mounts, path); succeed {
		op.Debugf("faking container stat path %#v", stat)
		return stat, nil
	}

	primaryTarget := resolvePathWithMountPoints(op, mounts, path)
	fs := primaryTarget.filterSpec

	var deviceID string
	var store string
	if primaryTarget.mountPoint.Destination == "/" {
		// Special case. / refers to container VMDK and not a volume vmdk.
		deviceID = vc.ContainerID
		store = constants.ContainerStoreName
	} else {
		deviceID = primaryTarget.mountPoint.Name
		store = constants.VolumeStoreName
	}

	stat, err = archiveProxy.StatPath(op, store, deviceID, fs)
	if err != nil {
		op.Errorf("error getting statpath: %s", err.Error())
		switch err := err.(type) {
		case *storage.StatPathNotFound:
			return nil, errors.ContainerResourceNotFoundError(vc.Name, "file or directory")
		case *storage.StatPathUnprocessableEntity:
			return nil, errors.InternalServerError("failed to process given path")
		default:
			return nil, errors.InternalServerError(err.Error())
		}
	}

	op.Debugf("container stat path %#v", stat)
	return stat, nil
}

//----------------------------------
// Docker cp utility
//----------------------------------

type ArchiveReader struct {
	mountPoint types.MountPoint
	filterSpec vicarchive.FilterSpec
	reader     io.ReadCloser
}
type ArchiveStreamReaderMap struct {
	prefixTrie *patricia.Trie
	op         trace.Operation
}

type ArchiveWriter struct {
	mountPoint types.MountPoint
	filterSpec vicarchive.FilterSpec
	writer     io.WriteCloser
	tarWriter  *tar.Writer
}

// ArchiveStreamWriterMap maps mount prefix to io.WriteCloser
type ArchiveStreamWriterMap struct {
	prefixTrie *patricia.Trie
	op         trace.Operation
	wg         *sync.WaitGroup
	errchan    chan error
}

// NewArchiveStreamWriterMap creates a new ArchiveStreamWriterMap.  The map contains all information
// needed to create a writers for every volume mounts for the container.  This includes the root
// volume of the container.
//
// mounts is the mount data from inspect
// containerDestPath is the destination path in the container
func NewArchiveStreamWriterMap(op trace.Operation, mounts []types.MountPoint, dest string) *ArchiveStreamWriterMap {
	writerMap := &ArchiveStreamWriterMap{}
	writerMap.prefixTrie = patricia.NewTrie()
	writerMap.op = op
	writerMap.errchan = make(chan error, 1)
	writerMap.wg = &sync.WaitGroup{}

	for _, m := range mounts {
		aw := ArchiveWriter{
			mountPoint: m,
			writer:     nil,
		}

		// If container destination path is part of this mount point's prefix, we must remove it and
		// add to the filterspec.  If the container destination path is "/" we do no stripping.
		//
		//	e.g. mount A at /mnt/A
		//
		//		cp /mnt cid:/mnt
		//
		// file data.txt from local /mnt/A/data.txt will come to the persona as A/data.txt.  We must
		// tell the storage portlayer to remove "A".
		//
		//	e.g. mount A at /mnt/A
		//
		//		cp / cid:/
		//
		// file data.txt from local /mnt/A/data.txt will come to the persona as mnt/A/data.txt.
		// Here, we must tell the portlayer to remove "mnt/A".  The key to determining whether to
		// strip "A" or "mnt/A" is based on the container destination path.
		isPrimary := !strings.Contains(aw.mountPoint.Destination, dest) || aw.mountPoint.Destination == dest
		aw.filterSpec = vicarchive.GenerateFilterSpec(dest, aw.mountPoint.Destination, isPrimary, vicarchive.CopyTo)

		writerMap.prefixTrie.Insert(patricia.Prefix(m.Destination), &aw)
	}

	return writerMap
}

// NewArchiveStreamReaderMap creates a new ArchiveStreamReaderMap.  After the call, it contains
// information to create readers for every volume mounts for the container
//
// mounts is the mount data from inspect
func NewArchiveStreamReaderMap(op trace.Operation, mounts []types.MountPoint, dest string) *ArchiveStreamReaderMap {
	readerMap := &ArchiveStreamReaderMap{}
	readerMap.prefixTrie = patricia.NewTrie()
	readerMap.op = op

	for _, m := range mounts {
		ar := ArchiveReader{
			mountPoint: m,
			reader:     nil,
		}

		// If the mount point is not the root file system, we must tell the portlayer to rebase the
		// files in the return tar stream with the mount point path since the volume does not know
		// the path it is mounted to.  It only knows it's root file system.
		//
		//	e.g. mount A at /mnt/A with a file data.txt in A
		//
		//	/mnt/A/data.txt		<-- from container point of view
		//	/data.txt			<-- from volume point of view
		//
		// Neither the volume nor the storage portlayer knows about /mnt/A.  The persona must tell
		// the portlayer to rebase all files from this volume to the /mnt/A/ in the final tar stream.
		cleanDest := vicarchive.Clean(dest, false)
		isPrimary := !strings.Contains(ar.mountPoint.Destination, cleanDest) || ar.mountPoint.Destination == cleanDest
		ar.filterSpec = vicarchive.GenerateFilterSpec(dest, ar.mountPoint.Destination, isPrimary, vicarchive.CopyFrom)

		readerMap.prefixTrie.Insert(patricia.Prefix(m.Destination), &ar)
	}

	return readerMap
}

// FindArchiveWriter finds the one writer that matches the asset name.  There should only be one
// stream this asset needs to be written to.
func (wm *ArchiveStreamWriterMap) FindArchiveWriter(containerDestPath, assetName string) (*ArchiveWriter, error) {
	defer trace.End(trace.Begin(""))

	var aw *ArchiveWriter
	var err error

	// go function used later for searching
	findPrefix := func(prefix patricia.Prefix, item patricia.Item) error {
		if _, ok := item.(*ArchiveWriter); !ok {
			return fmt.Errorf("item not ArchiveWriter")
		}

		aw, _ = item.(*ArchiveWriter)
		return nil
	}

	// Find the prefix for the final destination.  Final destination is the combination of container destination path
	// and the asset's name.  For example,
	//
	//	container destination path =	/
	//	asset name =					mnt/A/file.txt
	//	mount 1	=						/mnt/A
	//	mount prefix =					/mnt/A
	//
	// In the above example, mount prefxi can only be determined by combining both the container destination path and
	// the asset name, as the final destination includes a mounted volume.

	combinedPath := path.Join(containerDestPath, assetName)
	prefix := patricia.Prefix(combinedPath)
	err = wm.prefixTrie.VisitPrefixes(prefix, findPrefix)
	if err != nil {
		wm.op.Errorf(err.Error())
		return nil, fmt.Errorf("Failed to find a node for prefix %s: %s", containerDestPath, err.Error())
	}

	if aw == nil {
		return nil, fmt.Errorf("No archive writer found for container destination %s and asset name %s", containerDestPath, assetName)
	}

	return aw, nil
}

// WriterForAsset takes a destination path and subpath of the archive data and returns the
// appropriate writer for the two.  It's intention is to solve the case where there exist
// a mount point and another mount point within the first mount point.  For instance, the
// prefix map can have,
//
//		R/W -					/
//		mount 1 -				/mnt/a
//		mount 2 -				/mnt/a/b
//
//		case 1:
//				containerDestPath -			/mnt/a
//				archive header source -		b/file.txt
//
//			The correct writer would be the one corresponding to mount 2.
//
//		case 2:
//				containerDestPath -			/mnt/a
//				archive header source -		file.txt
//
//			The correct writer would be the one corresponding to mount 1.
//
//		case 3:
//				containerDestPath -			/
//				archive header source -		mnt/a/file.txt
//
//			The correct writer would be the one corresponding to mount 1
//
// As demonstrated above, the mount prefix and writer cannot be determined with just the
// container destination path.  It must be combined with the actual asset's name.
func (wm *ArchiveStreamWriterMap) WriterForAsset(proxy proxy.ArchiveProxy, cid, containerDestPath string, assetHeader tar.Header) (*tar.Writer, error) {
	defer trace.End(trace.Begin(assetHeader.Name))

	var err error

	aw, err := wm.FindArchiveWriter(containerDestPath, assetHeader.Name)
	if err != nil {
		return nil, err
	}

	// Perform the lazy initialization here.
	if aw.writer == nil || aw.tarWriter == nil {
		// lazy initialize.
		wm.op.Debugf("Lazily initializing import stream for %s", aw.mountPoint.Destination)
		var deviceID string
		var store string
		if aw.mountPoint.Destination == "/" {
			// Special case. / refers to container VMDK and not a volume vmdk.
			deviceID = cid
			store = constants.ContainerStoreName
		} else {
			deviceID = aw.mountPoint.Name
			store = constants.VolumeStoreName
		}
		rawWriter, err := proxy.ArchiveImportWriter(wm.op, store, deviceID, aw.filterSpec, wm.wg, wm.errchan)
		if err != nil {
			err = fmt.Errorf("Unable to initialize import stream writer for mount prefix %s", aw.mountPoint.Destination)
			wm.op.Errorf(err.Error())
			return nil, err
		}
		aw.writer = rawWriter
		aw.tarWriter = tar.NewWriter(rawWriter)
	}

	return aw.tarWriter, nil
}

// Close visits all the archive writer in the trie and closes the actual io.WritCloser
func (wm *ArchiveStreamWriterMap) Close(op trace.Operation) error {
	defer trace.End(trace.Begin(""))

	closeStream := func(prefix patricia.Prefix, item patricia.Item) error {
		if aw, ok := item.(*ArchiveWriter); ok && aw.writer != nil {
			aw.tarWriter.Close()
			aw.writer.Close()
			aw.tarWriter = nil
			aw.writer = nil
		}
		return nil
	}

	wm.prefixTrie.Visit(closeStream)

	// wait for all pl calls to return and close the channel
	go func() {
		wm.wg.Wait()
		close(wm.errchan)
	}()

	var err error
	// wait for all the streams to finish
	for result := range wm.errchan {
		if result != nil {
			err = result
			op.Errorf("Error received from portlayer for import streams: %s", result.Error())
		}
	}

	return err
}

// FindArchiveReaders finds all archive readers that are within the container source path.  For example,
//
//	mount A -			/mnt/A
//	mount B -			/mnt/B
//	mount AB -			/mnt/A/AB
//	base container -	/
//
//	container source path - /mnt/A
//
// For the above example, this function returns the readers for mount A and mount AB but not the
// readers for / or mount B.
func (rm *ArchiveStreamReaderMap) FindArchiveReaders(containerSourcePath string) ([]*ArchiveReader, error) {
	defer trace.End(trace.Begin(containerSourcePath))

	var nodes []*ArchiveReader
	var startingNode *ArchiveReader
	var err error
	var isMountPoint bool

	findStartingPrefix := func(prefix patricia.Prefix, item patricia.Item) error {
		if _, ok := item.(*ArchiveReader); !ok {
			return fmt.Errorf("item not ArchiveReader")
		}

		startingNode = item.(*ArchiveReader)
		return nil
	}

	walkPrefixSubtree := func(prefix patricia.Prefix, item patricia.Item) error {
		if _, ok := item.(*ArchiveReader); !ok {
			return fmt.Errorf("item not ArchiveReader")
		}

		ar, _ := item.(*ArchiveReader)
		nodes = append(nodes, ar)
		isMountPoint = ar.mountPoint.Destination != "/" &&
			(isMountPoint || strings.HasPrefix(containerSourcePath, ar.mountPoint.Destination))
		return nil
	}

	// Clean off any trailing periods from the path, such as `cp cid:/mnt/. -`
	// Including the periods in the prefix walk would not match with subvolume
	// mounts like /mnt/vol1 or /mnt/vol2.
	// Find all mounts for the sourcepath
	cleanPath := vicarchive.Clean(containerSourcePath, false)
	prefix := patricia.Prefix(cleanPath)
	err = rm.prefixTrie.VisitSubtree(prefix, walkPrefixSubtree)
	if err != nil {
		msg := fmt.Sprintf("Failed to find a node for prefix %s: %s", containerSourcePath, err.Error())
		rm.op.Errorf(msg)
		return nil, fmt.Errorf(msg)
	}

	// The above subtree walking MAY NOT find the starting prefix.  For example /etc will not find /.
	// Subtree only finds prefix that starts with /etc.  VisitPrefixes will find the starting prefix.
	// If the search was for /, then it will not find the starting node.  In that case, we grab the
	// first node in the slice.
	err = rm.prefixTrie.VisitPrefixes(prefix, findStartingPrefix)
	if err != nil {
		msg := fmt.Sprintf("Failed to find starting node for prefix %s: %s", containerSourcePath, err.Error())
		rm.op.Errorf(msg)
		return nil, fmt.Errorf(msg)
	}

	if startingNode != nil {
		found := false
		for _, node := range nodes {
			if node.mountPoint.Destination == startingNode.mountPoint.Destination {
				found = true
				break
			}
		}

		if !found {
			// prepend the starting node at the beginning
			nodes = append([]*ArchiveReader{startingNode}, nodes...)
		}
	} else if len(nodes) > 0 {
		startingNode = nodes[0]
	} else {
		msg := fmt.Sprintf("Failed to find starting node for prefix %s: %s", containerSourcePath, err.Error())
		rm.op.Errorf(msg)
		return nil, fmt.Errorf(msg)
	}

	// if the path is a mount path, we need to include the directory header of the actual mountpoint
	// to ensure the corrent permissions of the directory, eg docker cp cid:/mnt/vol1/ needs to include
	// 		header from /mnt/vol1 located on containerfs
	// 		data from /mnt/vol1/ located on deviceId vol1
	if isMountPoint && path.Base(containerSourcePath) != "." {
		rm.op.Debugf("%s is a mountpoint, getting dir permissions from parent", cleanPath)
		// find the parent node using VisitPrefixes
		parent := path.Dir(cleanPath)
		prefix = patricia.Prefix(parent)
		startingNode = nil
		err = rm.prefixTrie.VisitPrefixes(prefix, findStartingPrefix)
		if err != nil {
			msg := fmt.Sprintf("Failed to generate parent node for mountpoint %s: %s", parent, err.Error())
			rm.op.Errorf(msg)
			return nil, fmt.Errorf(msg)
		}

		var found bool
		if startingNode != nil {
			for _, node := range nodes {
				found = found || node.mountPoint.Destination == startingNode.mountPoint.Destination
			}
			if !found {
				nodes = append([]*ArchiveReader{startingNode}, nodes...)
			}
		}
	}

	err = rm.buildFilterSpec(containerSourcePath, nodes, startingNode)
	if err != nil {
		return nil, err
	}

	return nodes, nil
}

// ReadersForSourcePath returns all an array of io.Reader for all the readers within a container source path.
//		Example:
//			Reader 1 -				/mnt/A
//			Reader 2 -				/mnt/A/B
//
//			containerSourcePath -	/mnt/A
// In the above, both readers are within the the container source path.
func (rm *ArchiveStreamReaderMap) ReadersForSourcePath(proxy proxy.ArchiveProxy, cid, containerSourcePath string) ([]io.ReadCloser, error) {
	defer trace.End(trace.Begin(containerSourcePath))

	var streamReaders []io.ReadCloser

	nodes, err := rm.FindArchiveReaders(containerSourcePath)
	if err != nil {
		return nil, err
	}

	mounts := []string{}
	for _, node := range nodes {
		mounts = append(mounts, node.mountPoint.Destination)
	}

	// Create the io.Reader for those mounts if they haven't already been initialized
	for _, node := range nodes {
		if node.reader == nil {
			var store, deviceID string
			if node.mountPoint.Destination == "/" {
				// Special case. / refers to container VMDK and not a volume vmdk.
				store = constants.ContainerStoreName
				deviceID = cid
			} else {
				store = constants.VolumeStoreName
				deviceID = node.mountPoint.Name
			}

			rm.op.Infof("Lazily initializing export stream for %s [%s]", node.mountPoint.Name, node.mountPoint.Destination)
			reader, err := proxy.ArchiveExportReader(rm.op, store, "", deviceID, "", true, node.filterSpec)
			if err != nil {
				err = fmt.Errorf("Unable to initialize export stream reader for prefix %s", node.mountPoint.Destination)
				rm.op.Errorf(err.Error())
				return nil, err
			}
			rm.op.Infof("Lazy initialization created reader %#v", reader)
			streamReaders = append(streamReaders, reader)
		} else {
			streamReaders = append(streamReaders, node.reader)
		}
	}

	if len(nodes) == 0 {
		rm.op.Infof("Found no archive readers for %s", containerSourcePath)
	}

	return streamReaders, nil
}

// Close visits all the archive readers in the trie and closes the actual io.ReadCloser
func (rm *ArchiveStreamReaderMap) Close() {
	defer trace.End(trace.Begin(""))

	closeStream := func(prefix patricia.Prefix, item patricia.Item) error {
		if aw, ok := item.(*ArchiveReader); ok && aw.reader != nil {
			aw.reader.Close()
			aw.reader = nil
		}
		return nil
	}

	rm.prefixTrie.Visit(closeStream)
}

//  tryFakeStatPath tries to fake the statpath for path that targets the mountpoint or along the mountpoint
func tryFakeStatPath(mounts []types.MountPoint, target string) (*types.ContainerPathStat, bool) {
	isMountPathTarget := false

	for _, mount := range mounts {
		if strings.HasPrefix(mount.Destination, target) {
			isMountPathTarget = true
		}
	}

	// check to see if the path is a mount point, if so, return fake path
	if isMountPathTarget {
		return &types.ContainerPathStat{
			Name:       filepath.Base(target),
			Size:       int64(4096),
			Mode:       os.ModeDir,
			Mtime:      time.Now(),
			LinkTarget: "",
		}, true
	}

	return nil, false
}

// resolvePathWithMountPoints use mounts to generate a filter spec for the given path
func resolvePathWithMountPoints(op trace.Operation, mounts []types.MountPoint, path string) *ArchiveReader {
	var primaryTarget *ArchiveReader

	readerMap := NewArchiveStreamReaderMap(op, mounts, path)
	// #nosec: Errors unhandled.
	nodes, _ := readerMap.FindArchiveReaders(path)

	for _, node := range nodes {
		if strings.HasPrefix(path, node.mountPoint.Destination) &&
			(primaryTarget == nil || len(node.mountPoint.Destination) > len(primaryTarget.mountPoint.Destination)) {
			primaryTarget = node
		}
	}

	return primaryTarget
}

func (rm *ArchiveStreamReaderMap) buildFilterSpec(containerSourcePath string, nodes []*ArchiveReader, startingNode *ArchiveReader) error {

	mounts, foundNodes, err := rm.buildMountsAndNodes(startingNode.mountPoint.Destination, startingNode)
	if err != nil {
		return err
	}

	for _, node := range foundNodes {
		vicarchive.AddMountInclusionsExclusions(node.mountPoint.Destination, &node.filterSpec, mounts, containerSourcePath)
	}
	return nil
}

// buildMountsAndNodes returns the node pointers from the prefix tree as well as all mounts involved in the operation
func (rm *ArchiveStreamReaderMap) buildMountsAndNodes(path string, node *ArchiveReader) ([]string, []*ArchiveReader, error) {

	// NOTE(sflxn): We can modify this to make proper exclusions in the future. For now,
	// we assemble the list of mounts which are involved in the operation
	// and use the util.go function for generating all the needed information

	mounts := []string{}
	nodes := []*ArchiveReader{}
	childWalker := func(prefix patricia.Prefix, item patricia.Item) error {
		if _, ok := item.(*ArchiveReader); !ok {
			return fmt.Errorf("item not ArchiveReader")
		}

		ar, _ := item.(*ArchiveReader)
		mounts = append(mounts, ar.mountPoint.Destination)
		nodes = append(nodes, ar)
		return nil
	}

	// prefix = current node's mount path
	nodePrefix := patricia.Prefix(path)

	err := rm.prefixTrie.VisitSubtree(nodePrefix, childWalker)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to build exclusion filter for %s: %s", path, err.Error())
	}

	return mounts, nodes, nil
}
