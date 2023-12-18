/*
Copyright 2024 Intel Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hostpath

import (
	"errors"
	"fmt"
	"os"
	"path"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/volume/util/volumepathhandler"
	utilexec "k8s.io/utils/exec"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent/csi/state"
)

const (
	kb int64 = 1000
	mb int64 = kb * 1000

	// storageKind is the special parameter which requests
	// storage of a certain kind (only affects capacity checks).
	storageKind  = "kind"
	PVCName      = "csi.storage.k8s.io/pvc/name"
	PVCNameSpace = "csi.storage.k8s.io/pvc/namespace"
)

type hostPath struct {
	config     Config
	coreClient kubernetes.Interface
	// gRPC calls involving any of the fields below must be serialized
	// by locking this mutex before starting. Internal helper
	// functions assume that the mutex has been locked.
	mutex sync.Mutex
	state state.State
}

type Config struct {
	DriverName        string
	Endpoint          string
	ProxyEndpoint     string
	NodeID            string
	VendorVersion     string
	StateDir          string
	MaxVolumesPerNode int64
	MaxVolumeSize     int64
	AttachLimit       int64
	Ephemeral         bool
	ShowVersion       bool
	EnableAttach      bool
	EnableTopology    bool
	KubeConfig        string
	Master            string
	// EnableVolumeExpansion      bool
	// DisableControllerExpansion bool
	// DisableNodeExpansion       bool
	// MaxVolumeExpansionSizeNode int64
	CheckVolumeLifecycle bool
}

var (
// vendorVersion = "dev"
)

const (
// Extension with which snapshot files will be saved.
// snapshotExt = ".snap"
)

func NewHostPathDriver(cfg Config, c kubernetes.Interface, v versioned.Interface) (*hostPath, error) {
	if cfg.DriverName == "" {
		return nil, errors.New("no driver name provided")
	}

	if cfg.NodeID == "" {
		return nil, errors.New("no node id provided")
	}

	if cfg.Endpoint == "" {
		return nil, errors.New("no driver endpoint provided")
	}

	if c == nil {
		return nil, errors.New("k8s client is nil")
	}

	if err := os.MkdirAll(cfg.StateDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create dataRoot: %v", err)
	}

	klog.V(utils.DBG).Infof("Driver: %v ", cfg.DriverName)
	klog.V(utils.DBG).Infof("Version: %s", cfg.VendorVersion)

	s, err := state.New(path.Join(cfg.StateDir, "state.json"))
	if err != nil {
		return nil, err
	}

	hp := &hostPath{
		config:     cfg,
		state:      s,
		coreClient: c,
	}
	return hp, nil
}

func (hp *hostPath) Run() error {
	s := NewNonBlockingGRPCServer()
	// hp itself implements ControllerServer, NodeServer, and IdentityServer.
	s.Start(hp.config.Endpoint, hp, hp, hp)
	s.Wait()

	return nil
}

// getVolumePath returns the canonical path for hostpath volume
func (hp *hostPath) getVolumePath(volID string) (string, error) {
	vol, err := hp.state.GetVolumeByID(volID)
	if err != nil {
		return "", err
	}
	return vol.VolPath, nil
}

// createVolume allocates capacity, creates the directory for the hostpath volume, and
// adds the volume to the list.
//
// It returns the volume path or err if one occurs. That error is suitable as result of a gRPC call.
func (hp *hostPath) createVolume(volID, deviceID, path, name string, cap int64, volAccessType state.AccessType, kind string) (*state.Volume, error) {
	// Check for maximum available capacity
	if cap > hp.config.MaxVolumeSize {
		return nil, status.Errorf(codes.OutOfRange, "Requested capacity %d exceeds maximum allowed %d", cap, hp.config.MaxVolumeSize)
	}

	switch volAccessType {
	case state.MountAccess:
		err := os.MkdirAll(path, 0777)
		if err != nil {
			return nil, err
		}
	case state.BlockAccess:
		executor := utilexec.New()
		size := fmt.Sprintf("%dM", cap/mb)
		// Create a block file.
		_, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				out, err := executor.Command("fallocate", "-l", size, path).CombinedOutput()
				if err != nil {
					return nil, fmt.Errorf("failed to create block device: %v, %v", err, string(out))
				}
			} else {
				return nil, fmt.Errorf("failed to stat block device: %v, %v", path, err)
			}
		}

		// Associate block file with the loop device.
		volPathHandler := volumepathhandler.VolumePathHandler{}
		_, err = volPathHandler.AttachFileDevice(path)
		if err != nil {
			// Remove the block file because it'll no longer be used again.
			if err2 := os.Remove(path); err2 != nil {
				klog.Errorf("failed to cleanup block file %s: %v", path, err2)
			}
			return nil, fmt.Errorf("failed to attach device %v: %v", path, err)
		}
	default:
		return nil, fmt.Errorf("unsupported access type %v", volAccessType)
	}

	volume := state.Volume{
		VolID:         volID,
		VolName:       name,
		VolSize:       cap,
		VolPath:       path,
		VolAccessType: volAccessType,
		// Ephemeral:     ephemeral,
		Kind:     kind,
		DeviceID: deviceID,
	}
	klog.V(utils.DBG).Info("adding hostpath volume: %s = %+v", volID, volume)
	if err := hp.state.UpdateVolume(volume); err != nil {
		return nil, err
	}
	return &volume, nil
}

// deleteVolume deletes the directory for the hostpath volume.
func (hp *hostPath) deleteVolume(volID string) error {
	klog.V(utils.DBG).Info("starting to delete hostpath volume: %s", volID)

	vol, err := hp.state.GetVolumeByID(volID)
	if err != nil {
		// Return OK if the volume is not found.
		return nil
	}
	if len(vol.DeviceID) == 0 {
		return fmt.Errorf("empty device ID: %s", deviceID)
	}
	path := vol.VolPath
	if vol.VolAccessType == state.BlockAccess {
		volPathHandler := volumepathhandler.VolumePathHandler{}
		klog.V(utils.DBG).Info("deleting loop device for file %s if it exists", path)
		if err := volPathHandler.DetachFileDevice(path); err != nil {
			return fmt.Errorf("failed to remove loop device for file %s: %v", path, err)
		}
	}
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := hp.state.DeleteVolume(volID); err != nil {
		return err
	}
	klog.V(utils.DBG).Info("deleted hostpath volume: %s = %+v", volID, vol)
	return nil
}
