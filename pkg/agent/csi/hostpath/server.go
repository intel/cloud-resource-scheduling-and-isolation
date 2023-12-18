/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package hostpath

import (
	"sync"

	"google.golang.org/grpc"
	"k8s.io/klog/v2"

	"github.com/container-storage-interface/spec/lib/go/csi"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent/csi/endpoint"
)

func NewNonBlockingGRPCServer() *nonBlockingGRPCServer {
	return &nonBlockingGRPCServer{}
}

// NonBlocking server
type nonBlockingGRPCServer struct {
	wg      sync.WaitGroup
	server  *grpc.Server
	cleanup func()
}

func (s *nonBlockingGRPCServer) Start(endpoint string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer) {
	s.wg.Add(1)

	go s.serve(endpoint, ids, cs, ns)
}

func (s *nonBlockingGRPCServer) Wait() {
	s.wg.Wait()
}

func (s *nonBlockingGRPCServer) Stop() {
	s.server.GracefulStop()
	s.cleanup()
}

func (s *nonBlockingGRPCServer) ForceStop() {
	s.server.Stop()
	s.cleanup()
}

func (s *nonBlockingGRPCServer) serve(ep string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer) {
	listener, cleanup, err := endpoint.Listen(ep)
	if err != nil {
		klog.Fatalf("Failed to listen: %v", err)
	}

	server := grpc.NewServer()
	s.server = server
	s.cleanup = cleanup

	if ids != nil {
		klog.V(utils.DBG).Info("now register IdentityServer")
		csi.RegisterIdentityServer(server, ids)
	}
	if cs != nil {
		klog.V(utils.DBG).Info("now register ControllerServer")
		csi.RegisterControllerServer(server, cs)
	}
	if ns != nil {
		klog.V(utils.DBG).Info("now register NodeServer")
		csi.RegisterNodeServer(server, ns)
	}

	klog.V(utils.DBG).Info("Listening for connections on address: %#v", listener.Addr())

	err = server.Serve(listener)
	if err != nil {
		klog.Warning("Failed to serve listener, error: ", err)
	}

}
