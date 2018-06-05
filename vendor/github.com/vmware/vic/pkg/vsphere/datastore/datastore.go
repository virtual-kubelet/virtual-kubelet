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

package datastore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/tasks"
)

// Helper gives access to the datastore regardless of type (esx, esx + vc,
// or esx + vc + vsan).  Also wraps paths to a given root directory
type Helper struct {
	// The Datastore API likes everything in "path/to/thing" format.
	ds *object.Datastore

	s *session.Session

	// The FileManager API likes everything in "[dsname] path/to/thing" format.
	fm *object.FileManager

	// The datastore url (including root) in "[dsname] /path" format.
	RootURL object.DatastorePath
}

// NewDatastore returns a Datastore.
// ctx is a context,
// s is an authenticated session
// ds is the vsphere datastore
// rootdir is the top level directory to root all data.  If root does not exist,
// it will be created.  If it already exists, NOOP. This cannot be empty.
func NewHelper(ctx context.Context, s *session.Session, ds *object.Datastore, rootdir string) (*Helper, error) {
	op := trace.FromContext(ctx, "NewHelper")

	d := &Helper{
		ds: ds,
		s:  s,
		fm: object.NewFileManager(s.Vim25()),
	}

	if path.IsAbs(rootdir) {
		rootdir = rootdir[1:]
	}

	if err := d.mkRootDir(op, rootdir); err != nil {
		op.Infof("error creating root directory %s: %s", rootdir, err)
		return nil, err
	}

	if d.RootURL.Path == "" {
		return nil, fmt.Errorf("failed to create root directory")
	}

	op.Infof("Datastore path is %s", d.RootURL.String())
	return d, nil
}

func NewHelperFromURL(ctx context.Context, s *session.Session, u *url.URL) (*Helper, error) {
	fm := object.NewFileManager(s.Vim25())
	vsDs, err := s.Finder.DatastoreOrDefault(ctx, u.Host)
	if err != nil {
		return nil, err
	}

	d := &Helper{
		ds: vsDs,
		s:  s,
		fm: fm,
	}

	d.RootURL.FromString(u.Path)

	return d, nil
}

func NewHelperFromSession(ctx context.Context, s *session.Session) *Helper {
	return &Helper{
		ds: s.Datastore,
		s:  s,
		fm: object.NewFileManager(s.Vim25()),
	}
}

// GetDatastores returns a map of datastores given a map of names and urls
func GetDatastores(ctx context.Context, s *session.Session, dsURLs map[string]*url.URL) (map[string]*Helper, error) {
	stores := make(map[string]*Helper)

	for name, dsURL := range dsURLs {
		d, err := NewHelperFromURL(ctx, s, dsURL)
		if err != nil {
			return nil, err
		}

		stores[name] = d
	}

	return stores, nil
}

func (d *Helper) Summary(ctx context.Context) (*types.DatastoreSummary, error) {

	var mds mo.Datastore
	if err := d.ds.Properties(ctx, d.ds.Reference(), []string{"info", "summary"}, &mds); err != nil {
		return nil, err
	}

	return &mds.Summary, nil
}

func mkdir(op trace.Operation, sess *session.Session, fm *object.FileManager, createParentDirectories bool, path string) (string, error) {
	op.Infof("Creating directory %s", path)

	if err := fm.MakeDirectory(op, path, sess.Datacenter, createParentDirectories); err != nil {
		if soap.IsSoapFault(err) {
			soapFault := soap.ToSoapFault(err)
			if _, ok := soapFault.VimFault().(types.FileAlreadyExists); ok {
				op.Debugf("File already exists: %s", path)
				return "", os.ErrExist
			}
		}
		op.Debugf("Creating %s error: %s", path, err)
		return "", err
	}

	return path, nil
}

// Mkdir creates directories.
func (d *Helper) Mkdir(ctx context.Context, createParentDirectories bool, dirs ...string) (string, error) {
	op := trace.FromContext(ctx, "Mkdir")

	return mkdir(op, d.s, d.fm, createParentDirectories, path.Join(d.RootURL.String(), path.Join(dirs...)))
}

// Ls returns a list of dirents at the given path (relative to root)
//
// A note aboutpaths and the datastore browser.
// None of these work paths work
// r, err := ds.Ls(ctx, "ds:///vmfs/volumes/vsan:52a67632ac3497a3-411916fd50bedc27/0ea65357-0494-d42d-2ede-000c292dc5b5")
// r, err := ds.Ls(ctx, "[vsanDatastore] ds:///vmfs/volumes/vsan:52a67632ac3497a3-411916fd50bedc27/")
// r, err := ds.Ls(ctx, "[vsanDatastore] //vmfs/volumes/vsan:52a67632ac3497a3-411916fd50bedc27/")
// r, err := ds.Ls(ctx, "[] ds:///vmfs/volumes/vsan:52a67632ac3497a3-411916fd50bedc27/0ea65357-0494-d42d-2ede-000c292dc5b5")
// r, err := ds.Ls(ctx, "[] /vmfs/volumes/vsan:52a67632ac3497a3-411916fd50bedc27/0ea65357-0494-d42d-2ede-000c292dc5b5")
// r, err := ds.Ls(ctx, "[] ../vmfs/volumes/vsan:52a67632ac3497a3-411916fd50bedc27/0ea65357-0494-d42d-2ede-000c292dc5b5")
// r, err := ds.Ls(ctx, "[] ./vmfs/volumes/vsan:52a67632ac3497a3-411916fd50bedc27/0ea65357-0494-d42d-2ede-000c292dc5b5")
// r, err := ds.Ls(ctx, "[52a67632ac3497a3-411916fd50bedc27] /0ea65357-0494-d42d-2ede-000c292dc5b5")
// r, err := ds.Ls(ctx, "[vsan:52a67632ac3497a3-411916fd50bedc27] /0ea65357-0494-d42d-2ede-000c292dc5b5")
// r, err := ds.Ls(ctx, "[vsan:52a67632ac3497a3-411916fd50bedc27] 0ea65357-0494-d42d-2ede-000c292dc5b5")
// r, err := ds.Ls(ctx, "[vsanDatastore] /vmfs/volumes/vsan:52a67632ac3497a3-411916fd50bedc27/0ea65357-0494-d42d-2ede-000c292dc5b5")

// The only URI that works on VC + VSAN.
// r, err := ds.Ls(ctx, "[vsanDatastore] /0ea65357-0494-d42d-2ede-000c292dc5b5")
//
func (d *Helper) Ls(ctx context.Context, p string) (*types.HostDatastoreBrowserSearchResults, error) {
	spec := types.HostDatastoreBrowserSearchSpec{
		MatchPattern: []string{"*"},
		Details: &types.FileQueryFlags{
			FileType:  true,
			FileOwner: types.NewBool(true),
		},
	}

	b, err := d.ds.Browser(ctx)
	if err != nil {
		return nil, err
	}

	task, err := b.SearchDatastore(ctx, path.Join(d.RootURL.String(), p), &spec)
	if err != nil {
		return nil, err
	}

	info, err := task.WaitForResult(ctx, nil)
	if err != nil {
		return nil, err
	}

	res := info.Result.(types.HostDatastoreBrowserSearchResults)
	return &res, nil
}

// LsDirs returns a list of dirents at the given path (relative to root)
func (d *Helper) LsDirs(ctx context.Context, p string) (*types.ArrayOfHostDatastoreBrowserSearchResults, error) {
	spec := &types.HostDatastoreBrowserSearchSpec{
		MatchPattern: []string{"*"},
		Details: &types.FileQueryFlags{
			FileType:  true,
			FileOwner: types.NewBool(true),
		},
	}

	b, err := d.ds.Browser(ctx)
	if err != nil {
		return nil, err
	}

	task, err := b.SearchDatastoreSubFolders(ctx, path.Join(d.RootURL.String(), p), spec)
	if err != nil {
		return nil, err
	}

	info, err := task.WaitForResult(ctx, nil)
	if err != nil {
		return nil, err
	}

	res := info.Result.(types.ArrayOfHostDatastoreBrowserSearchResults)
	return &res, nil
}

func (d *Helper) Upload(ctx context.Context, r io.Reader, pth string) error {
	return d.ds.Upload(ctx, r, path.Join(d.RootURL.Path, pth), &soap.DefaultUpload)
}

func (d *Helper) Download(ctx context.Context, pth string) (io.ReadCloser, error) {
	rc, _, err := d.ds.Download(ctx, path.Join(d.RootURL.Path, pth), &soap.DefaultDownload)
	return rc, err
}

func (d *Helper) Stat(ctx context.Context, pth string) (types.BaseFileInfo, error) {
	i, err := d.ds.Stat(ctx, path.Join(d.RootURL.Path, pth))
	if err != nil {
		switch err.(type) {
		case object.DatastoreNoSuchDirectoryError:
			return nil, os.ErrNotExist
		default:
			return nil, err
		}
	}

	return i, nil
}

func (d *Helper) Mv(ctx context.Context, fromPath, toPath string) error {
	op := trace.FromContext(ctx, "Mv")

	from := path.Join(d.RootURL.String(), fromPath)
	to := path.Join(d.RootURL.String(), toPath)
	op.Infof("Moving %s to %s", from, to)
	err := tasks.Wait(ctx, func(context.Context) (tasks.Task, error) {
		return d.fm.MoveDatastoreFile(ctx, from, d.s.Datacenter, to, d.s.Datacenter, true)
	})

	return err
}

func (d *Helper) Rm(ctx context.Context, pth string) error {
	op := trace.FromContext(ctx, "Rm")

	f := path.Join(d.RootURL.String(), pth)
	op.Infof("Removing %s", pth)
	return d.ds.NewFileManager(d.s.Datacenter, true).Delete(ctx, f) // TODO: NewHelper should create the DatastoreFileManager
}

func (d *Helper) IsVSAN(ctx context.Context) bool {
	// #nosec: Errors unhandled.
	dsType, _ := d.ds.Type(ctx)
	return dsType == types.HostFileSystemVolumeFileSystemTypeVsan
}

// This creates the root directory in the datastore and sets the rooturl and
// rootdir in the datastore struct so we can reuse it for other routines.  This
// handles vsan + vc, vsan + esx, and esx.  The URI conventions are not the
// same for each and this tries to create the directory and stash the relevant
// result so the URI doesn't need to be recomputed for every datastore
// operation.
func (d *Helper) mkRootDir(op trace.Operation, rootdir string) error {
	if rootdir == "" {
		return fmt.Errorf("root directory is empty")
	}

	if path.IsAbs(rootdir) {
		return fmt.Errorf("root directory (%s) must not be an absolute path", rootdir)
	}

	// Handle vsan
	// Vsan will complain if the root dir exists.  Just call it directly and
	// swallow the error if it's already there.
	if d.IsVSAN(op) {
		comps := strings.Split(rootdir, "/")

		nm := object.NewDatastoreNamespaceManager(d.s.Vim25())

		// This returns the vmfs path (including the datastore and directory
		// UUIDs).  Use the directory UUID in future operations because it is
		// the stable path which we can use regardless of vsan state.
		uuid, err := nm.CreateDirectory(op, d.ds, comps[0], "")
		if err != nil {
			if !soap.IsSoapFault(err) {
				return err
			}

			soapFault := soap.ToSoapFault(err)
			if _, ok := soapFault.VimFault().(types.FileAlreadyExists); !ok {
				return err
			}

			// XXX UGLY HACK until we move this into the installer.  Use the
			// display name if the dir exists since we can't get the UUID after the
			// directory is created.
			uuid = comps[0]
			err = nil
		}

		rootdir = path.Join(path.Base(uuid), path.Join(comps[1:]...))
	}

	rooturl := d.ds.Path(rootdir)

	// create the rest of the root dir in case of vSAN, otherwise
	// create the full path
	if _, err := mkdir(op, d.s, d.fm, true, rooturl); err != nil {
		if !os.IsExist(err) {
			return err
		}

		op.Infof("datastore root %s already exists", rooturl)
	}

	d.RootURL.FromString(rooturl)
	return nil
}

func PathFromString(dsp string) (*object.DatastorePath, error) {
	var p object.DatastorePath
	if !p.FromString(dsp) {
		return nil, errors.New(dsp + " not a datastore path")
	}

	return &p, nil
}

// Parse the datastore format ([datastore1] /path/to/thing) to groups.
var datastoreFormat = regexp.MustCompile(`^\[([\w\d\(\)-_\.\s]+)\]`)
var pathFormat = regexp.MustCompile(`\s([\/\w-_\.]*$)`)

// Converts `[datastore] /path` to ds:// URL
func ToURL(ds string) (*url.URL, error) {
	u := new(url.URL)
	var matches []string
	if matches = datastoreFormat.FindStringSubmatch(ds); len(matches) != 2 {
		return nil, fmt.Errorf("Ambiguous datastore hostname format encountered from input: %s.", ds)
	}
	u.Host = matches[1]
	if matches = pathFormat.FindStringSubmatch(ds); len(matches) != 2 {
		return nil, fmt.Errorf("Ambiguous datastore path format encountered from input: %s.", ds)
	}

	u.Path = path.Clean(matches[1])
	u.Scheme = "ds"

	return u, nil
}

// Converts ds:// URL for datastores to datastore format ([datastore1] /path/to/thing)
func URLtoDatastore(u *url.URL) (string, error) {
	scheme := "ds"
	if u.Scheme != scheme {
		return "", fmt.Errorf("url (%s) is not a datastore", u.String())
	}
	return fmt.Sprintf("[%s] %s", u.Host, u.Path), nil
}

// TestName builds a unique datastore name
func TestName(suffix string) string {
	return uuid.New().String()[0:16] + "-" + suffix
}
