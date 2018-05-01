// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
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

package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"

	"github.com/google/uuid"

	"github.com/vmware/vic/lib/portlayer/exec2"
	"github.com/vmware/vic/lib/portlayer/exec2/remote"
)

type PortLayerRPCServer struct {
}

// A Handle can be anything, so this takes advantage of that by creating a sparse handle
// to send to the client and using that sparse handle as a key in a hashtable which points to
// rich handles created by the HandleFactory
type SparseHandle exec2.Handle

var lcTarget exec2.ContainerLifecycle

//var lcQuery exec2.ContainerQuery
var handles map[SparseHandle]exec2.Handle

func init() {
	pl := &exec2.PortLayerVsphere{}
	pl.Init(nil, &exec2.BasicHandleFactory{})
	lcTarget = pl
	//lcQuery = lcTarget
	handles = make(map[SparseHandle]exec2.Handle)
}

func main() {
	gob.Register(uuid.New())
	rpcServer := new(PortLayerRPCServer)
	rpc.Register(rpcServer)
	rpc.HandleHTTP()
	// #nosec: Binds to all network interfaces
	l, e := net.Listen("tcp", ":1234")
	if e != nil {
		log.Fatal("listen error:", e)
	}
	fmt.Println("Server listening")
	http.Serve(l, nil)
}

// A sparse handle is simply a random string
func newSparseHandle() SparseHandle {
	return SparseHandle(uuid.New())
}

func createSparseHandle(handle exec2.Handle) SparseHandle {
	key := newSparseHandle()
	handles[key] = handle
	return key
}

func resolveSparseHandle(handle SparseHandle) exec2.Handle {
	return handles[handle]
}

func refreshHandle(result *exec2.Handle, oldHandle SparseHandle, newHandle exec2.Handle, err error) error {
	*result = createSparseHandle(newHandle)
	delete(handles, oldHandle)
	return err
}

func (*PortLayerRPCServer) CreateContainer(args remote.CreateArgs, result *exec2.Handle) error {
	handle, err := lcTarget.CreateContainer(args.Name)
	*result = createSparseHandle(handle)
	return err
}

func (*PortLayerRPCServer) GetHandle(cid exec2.ID, result *exec2.Handle) error {
	handle, err := lcTarget.GetHandle(cid)
	*result = createSparseHandle(handle)
	return err
}

func (*PortLayerRPCServer) CopyTo(args remote.CopyToArgs, result *exec2.Handle) error {
	handle := resolveSparseHandle(args.Handle)
	newHandle, err := lcTarget.CopyTo(handle, args.TargetDir, args.Fname, args.Perms, args.Data)
	return refreshHandle(result, handle, newHandle, err)
}

func (*PortLayerRPCServer) SetEntryPoint(args remote.SetEntryPointArgs, result *exec2.Handle) error {
	handle := resolveSparseHandle(args.Handle)
	newHandle, err := lcTarget.SetEntryPoint(handle, args.WorkDir, args.ExecPath, args.Args)
	return refreshHandle(result, handle, newHandle, err)
}

func (*PortLayerRPCServer) SetLimits(args remote.SetLimitsArgs, result *exec2.Handle) error {
	handle := resolveSparseHandle(args.Handle)
	newHandle, err := lcTarget.SetLimits(handle, args.MemoryMb, args.CPUMhz)
	return refreshHandle(result, handle, newHandle, err)
}

func (*PortLayerRPCServer) SetRunState(args remote.SetRunStateArgs, result *exec2.Handle) error {
	handle := resolveSparseHandle(args.Handle)
	newHandle, err := lcTarget.SetRunState(handle, args.RunState)
	return refreshHandle(result, handle, newHandle, err)
}

func (*PortLayerRPCServer) Commit(args remote.CommitArgs, result *exec2.ID) error {
	cid, err := lcTarget.Commit(resolveSparseHandle(args.Handle))
	*result = cid
	return err
}

func (*PortLayerRPCServer) DestroyContainer(cid exec2.ID, result *exec2.ID) error {
	err := lcTarget.DestroyContainer(cid)
	*result = cid
	return err
}
