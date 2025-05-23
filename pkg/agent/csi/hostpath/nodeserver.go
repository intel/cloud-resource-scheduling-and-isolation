/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package hostpath

import (
	"fmt"
	"os"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/volume/util/volumepathhandler"
	"k8s.io/utils/mount"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent/csi/state"
)

const (
	TopologyKeyNode = "topology.hostpath.csi/node"

	failedPreconditionAccessModeConflict = "volume uses SINGLE_NODE_SINGLE_WRITER access mode and is already mounted at a different target path"
)

func (hp *hostPath) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	// Check arguments
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability missing in request")
	}
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	targetPath := req.GetTargetPath()

	if req.GetVolumeCapability().GetBlock() != nil &&
		req.GetVolumeCapability().GetMount() != nil {
		return nil, status.Error(codes.InvalidArgument, "cannot have both block and mount access type")
	}

	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.mutex.Lock()
	defer hp.mutex.Unlock()

	mounter := mount.New("")

	vol, err := hp.state.GetVolumeByID(req.GetVolumeId())
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	if hasSingleNodeSingleWriterAccessMode(req) && isMountedElsewhere(req, vol) {
		return nil, status.Error(codes.FailedPrecondition, failedPreconditionAccessModeConflict)
	}

	// if !ephemeralVolume {
	if vol.Staged.Empty() {
		return nil, status.Errorf(codes.FailedPrecondition, "volume %q must be staged before publishing", vol.VolID)
	}
	if !vol.Staged.Has(req.GetStagingTargetPath()) {
		return nil, status.Errorf(codes.InvalidArgument, "volume %q was staged at %v, not %q", vol.VolID, vol.Staged, req.GetStagingTargetPath())
	}
	// }

	if req.GetVolumeCapability().GetBlock() != nil {
		if vol.VolAccessType != state.BlockAccess {
			return nil, status.Error(codes.InvalidArgument, "cannot publish a non-block volume as block volume")
		}

		volPathHandler := volumepathhandler.VolumePathHandler{}

		// Get loop device from the volume path.
		loopDevice, err := volPathHandler.GetLoopDevice(vol.VolPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get the loop device: %w", err)
		}

		// Check if the target path exists. Create if not present.
		_, err = os.Lstat(targetPath)
		if os.IsNotExist(err) {
			if err = makeFile(targetPath); err != nil {
				return nil, fmt.Errorf("failed to create target path: %w", err)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("failed to check if the target block file exists: %w", err)
		}

		// Check if the target path is already mounted. Prevent remounting.
		notMount, err := mount.IsNotMountPoint(mounter, targetPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("error checking path %s for mount: %w", targetPath, err)
			}
			notMount = true
		}
		if !notMount {
			// It's already mounted.
			klog.V(utils.DBG).Info("Skipping bind-mounting subpath %s: already mounted", targetPath)
			return &csi.NodePublishVolumeResponse{}, nil
		}

		options := []string{"bind"}
		if err := mounter.Mount(loopDevice, targetPath, "", options); err != nil {
			return nil, fmt.Errorf("failed to mount block device: %s at %s: %w", loopDevice, targetPath, err)
		}
	} else if req.GetVolumeCapability().GetMount() != nil {
		if vol.VolAccessType != state.MountAccess {
			return nil, status.Error(codes.InvalidArgument, "cannot publish a non-mount volume as mount volume")
		}

		notMnt, err := mount.IsNotMountPoint(mounter, targetPath)
		if err != nil {
			if os.IsNotExist(err) {
				if err = os.Mkdir(targetPath, 0750); err != nil {
					return nil, fmt.Errorf("create target path: %w", err)
				}
				notMnt = true
			} else {
				return nil, fmt.Errorf("check target path: %w", err)
			}
		}

		if !notMnt {
			return &csi.NodePublishVolumeResponse{}, nil
		}

		fsType := req.GetVolumeCapability().GetMount().GetFsType()

		deviceId := ""
		if req.GetPublishContext() != nil {
			deviceId = req.GetPublishContext()[deviceID]
		}

		readOnly := req.GetReadonly()
		volumeId := req.GetVolumeId()
		attrib := req.GetVolumeContext()
		mountFlags := req.GetVolumeCapability().GetMount().GetMountFlags()

		klog.V(utils.DBG).Infof("target %v\nfstype %v\ndevice %v\nreadonly %v\nvolumeId %v\nattributes %v\nmountflags %v\n",
			targetPath, fsType, deviceId, readOnly, volumeId, attrib, mountFlags)

		options := []string{"bind"}
		if readOnly {
			options = append(options, "ro")
		}
		path, ok := attrib[utils.AnnoVolPath]
		if !ok {
			return nil, fmt.Errorf("cannot get mount path with device path: %s", attrib[utils.AnnoVolPath])
		}
		klog.V(utils.DBG).Infof("Mounting device %s at %s", path, targetPath)

		if err := mounter.Mount(path, targetPath, "", options); err != nil {
			var errList strings.Builder
			errList.WriteString(err.Error())
			return nil, fmt.Errorf("failed to mount device: %s at %s: %s", path, targetPath, errList.String())
		}
	}

	vol.NodeID = hp.config.NodeID
	vol.Published.Add(targetPath)
	if err := hp.state.UpdateVolume(vol); err != nil {
		return nil, err
	}
	return &csi.NodePublishVolumeResponse{}, nil
}

func (hp *hostPath) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {

	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	targetPath := req.GetTargetPath()
	volumeID := req.GetVolumeId()

	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.mutex.Lock()
	defer hp.mutex.Unlock()

	vol, err := hp.state.GetVolumeByID(volumeID)
	if err != nil {
		return nil, err
	}

	if !vol.Published.Has(targetPath) {
		klog.V(utils.DBG).Info("Volume %q is not published at %q, nothing to do.", volumeID, targetPath)
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}

	// Unmount only if the target path is really a mount point.
	if notMnt, err := mount.IsNotMountPoint(mount.New(""), targetPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("check target path: %w", err)
		}
	} else if !notMnt {
		// Unmounting the image or filesystem.
		err = mount.New("").Unmount(targetPath)
		if err != nil {
			return nil, fmt.Errorf("unmount target path: %w", err)
		}
	}
	// Delete the mount point.
	// Does not return error for non-existent path, repeated calls OK for idempotency.
	if err = os.RemoveAll(targetPath); err != nil {
		return nil, fmt.Errorf("remove target path: %w", err)
	}
	klog.V(utils.DBG).Info("hostpath: volume %s has been unpublished.", targetPath)

	vol.Published.Remove(targetPath)
	if err := hp.state.UpdateVolume(vol); err != nil {
		return nil, err
	}
	// }

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (hp *hostPath) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {

	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	stagingTargetPath := req.GetStagingTargetPath()
	if stagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume Capability missing in request")
	}

	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.mutex.Lock()
	defer hp.mutex.Unlock()

	vol, err := hp.state.GetVolumeByID(req.VolumeId)
	if err != nil {
		return nil, err
	}

	if hp.config.EnableAttach && !vol.Attached {
		return nil, status.Errorf(codes.Internal, "ControllerPublishVolume must be called on volume '%s' before staging on node",
			vol.VolID)
	}

	if vol.Staged.Has(stagingTargetPath) {
		klog.V(utils.DBG).Info("Volume %q is already staged at %q, nothing to do.", req.VolumeId, stagingTargetPath)
		return &csi.NodeStageVolumeResponse{}, nil
	}

	if !vol.Staged.Empty() {
		return nil, status.Errorf(codes.FailedPrecondition, "volume %q is already staged at %v", req.VolumeId, vol.Staged)
	}

	vol.Staged.Add(stagingTargetPath)
	if err := hp.state.UpdateVolume(vol); err != nil {
		return nil, err
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (hp *hostPath) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {

	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	stagingTargetPath := req.GetStagingTargetPath()
	if stagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.mutex.Lock()
	defer hp.mutex.Unlock()

	vol, err := hp.state.GetVolumeByID(req.VolumeId)
	if err != nil {
		return nil, err
	}

	if !vol.Staged.Has(stagingTargetPath) {
		klog.V(utils.DBG).Info("Volume %q is not staged at %q, nothing to do.", req.VolumeId, stagingTargetPath)
		return &csi.NodeUnstageVolumeResponse{}, nil
	}

	if !vol.Published.Empty() {
		return nil, status.Errorf(codes.Internal, "volume %q is still published at %q on node %q", vol.VolID, vol.Published, vol.NodeID)
	}
	vol.Staged.Remove(stagingTargetPath)
	if err := hp.state.UpdateVolume(vol); err != nil {
		return nil, err
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (hp *hostPath) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	c := []*csi.NodeServiceCapability{
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
				},
			},
		},
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_VOLUME_CONDITION,
				},
			},
		},
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
				},
			},
		},
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
				},
			},
		},
	}
	return &csi.NodeGetCapabilitiesResponse{Capabilities: c}, nil
}

func (hp *hostPath) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	resp := &csi.NodeGetInfoResponse{
		NodeId:            hp.config.NodeID,
		MaxVolumesPerNode: hp.config.MaxVolumesPerNode,
	}

	if hp.config.EnableTopology {
		resp.AccessibleTopology = &csi.Topology{
			Segments: map[string]string{TopologyKeyNode: hp.config.NodeID},
		}
	}

	if hp.config.AttachLimit > 0 {
		resp.MaxVolumesPerNode = hp.config.AttachLimit
	}

	return resp, nil
}

func (hp *hostPath) NodeGetVolumeStats(ctx context.Context, in *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	if len(in.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}
	if len(in.GetVolumePath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume Path not provided")
	}

	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.mutex.Lock()
	defer hp.mutex.Unlock()

	volume, err := hp.state.GetVolumeByID(in.GetVolumeId())
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(in.GetVolumePath()); err != nil {
		return nil, status.Errorf(codes.NotFound, "Could not get file information from %s: %+v", in.GetVolumePath(), err)
	}

	healthy, msg := hp.doHealthCheckInNodeSide(in.GetVolumeId())
	klog.V(utils.DBG).Infof("Healthy state: %+v Volume: %+v", volume.VolName, healthy)
	available, capacity, used, inodes, inodesFree, inodesUsed, err := getPVStats(in.GetVolumePath())
	if err != nil {
		return nil, fmt.Errorf("get volume stats failed: %w", err)
	}

	klog.V(utils.DBG).Infof("Capacity: %+v Used: %+v Available: %+v Inodes: %+v Free inodes: %+v Used inodes: %+v", capacity, used, available, inodes, inodesFree, inodesUsed)
	return &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			{
				Available: available,
				Used:      used,
				Total:     capacity,
				Unit:      csi.VolumeUsage_BYTES,
			}, {
				Available: inodesFree,
				Used:      inodesUsed,
				Total:     inodes,
				Unit:      csi.VolumeUsage_INODES,
			},
		},
		VolumeCondition: &csi.VolumeCondition{
			Abnormal: !healthy,
			Message:  msg,
		},
	}, nil
}

// NodeExpandVolume is only implemented so the driver can be used for e2e testing.
func (hp *hostPath) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return &csi.NodeExpandVolumeResponse{}, nil
}

// makeFile ensures that the file exists, creating it if necessary.
// The parent directory must exist.
func makeFile(pathname string) error {
	f, err := os.OpenFile(pathname, os.O_CREATE, os.FileMode(0644))
	if err != nil {
		klog.Warning("Failed to open file, error: ", err)
	}
	defer f.Close()
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	return nil
}

// hasSingleNodeSingleWriterAccessMode checks if the publish request uses the
// SINGLE_NODE_SINGLE_WRITER access mode.
func hasSingleNodeSingleWriterAccessMode(req *csi.NodePublishVolumeRequest) bool {
	accessMode := req.GetVolumeCapability().GetAccessMode()
	return accessMode != nil && accessMode.GetMode() == csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER
}

// isMountedElsewhere checks if the volume to publish is mounted elsewhere on
// the node.
func isMountedElsewhere(req *csi.NodePublishVolumeRequest, vol state.Volume) bool {
	for _, targetPath := range vol.Published {
		if targetPath != req.GetTargetPath() {
			return true
		}
	}
	return false
}
