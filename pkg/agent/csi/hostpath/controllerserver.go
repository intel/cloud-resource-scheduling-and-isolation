/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package hostpath

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/uuid"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	kubeerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent/csi/state"
)

const (
	deviceID = "deviceID"
)

func UpdateVolumeDeviceMap(cm *v1.ConfigMap, needCreate bool, DeviceInUse string, hp *hostPath) error {
	if needCreate {
		cmName := fmt.Sprintf("%s-%s", utils.DeviceIDConfigMap, hp.config.NodeID)
		data := map[string]string{
			utils.DeviceInUse: DeviceInUse,
		}
		cm := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: utils.IoiNamespace,
				Name:      cmName,
				Labels:    map[string]string{"app": utils.DeviceIDConfigMap},
			},
			Data: data,
		}
		cm, err := hp.coreClient.CoreV1().ConfigMaps(utils.IoiNamespace).Create(context.Background(), cm, metav1.CreateOptions{})
		if err != nil {
			klog.ErrorS(err, "Failed to create ConfigMap volume-device-map", "node", hp.config.NodeID)
			return err
		}
		klog.V(utils.DBG).InfoS("ConfigMap volume-device-map has been created", "node", hp.config.NodeID, "data", cm.Data)
	} else {
		cm = cm.DeepCopy()
		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}
		cm.Data[utils.DeviceInUse] = DeviceInUse
		configmap, err := hp.coreClient.CoreV1().ConfigMaps(utils.IoiNamespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
		if err != nil {
			klog.ErrorS(err, "Failed to create ConfigMap volume-device-map", "node", hp.config.NodeID)
			return err
		}
		klog.V(utils.DBG).InfoS("ConfigMap volume-device-map has been updated", "node", hp.config.NodeID, "data", configmap.Data)
	}
	return nil
}

func CalDeviceInUse(curDeviceInUse string, deviceId string, inUse bool, volumes []state.Volume) (bool, string, error) {
	devInUse, err := utils.JsonToStrings(curDeviceInUse)
	if err != nil {
		klog.Warning("failed to convert device in use to string:", err)
		devInUse = []string{}
	}
	// change DeviceInUse
	if inUse {
		for _, dev := range devInUse {
			if dev == deviceId {
				return false, curDeviceInUse, nil // no change
			}
		}
		devInUse = append(devInUse, deviceId)
	} else {
		for _, volume := range volumes {
			if volume.DeviceID == deviceId {
				return false, curDeviceInUse, nil // no change
			}
		}
		for i, dev := range devInUse {
			if dev == deviceId {
				devInUse = append(devInUse[:i], devInUse[i+1:]...)
				break
			}
		}
	}
	newDeviceInUse, err := utils.StringsToJson(devInUse)
	if err != nil {
		return false, newDeviceInUse, fmt.Errorf("failed to convert device in use to json: %v", err)
	}
	return true, newDeviceInUse, nil
}

func UpdateDeviceInUse(deviceId string, inUse bool, hp *hostPath) error {
	needCreate := false
	curDeviceInUse := "[]"
	// 1. get configmap
	cm, err := hp.coreClient.CoreV1().ConfigMaps(utils.IoiNamespace).Get(context.TODO(), fmt.Sprintf("%s-%s", utils.DeviceIDConfigMap, hp.config.NodeID), metav1.GetOptions{})
	if err != nil && kubeerr.IsNotFound(err) {
		needCreate = true
	} else if err != nil {
		return fmt.Errorf("failed to get ConfigMap: %s due to %s", utils.DeviceIDConfigMap, err.Error())
	}
	if v, ok := cm.Data[utils.DeviceInUse]; ok {
		curDeviceInUse = v
	}
	// 2. change it DeviceInUse
	volumes := hp.state.GetVolumes()
	klog.V(utils.INF).Infof("volumes: %v", volumes)
	isUpdateCr, newDeviceInUse, err := CalDeviceInUse(curDeviceInUse, deviceId, inUse, volumes)
	if err != nil {
		klog.Warningf("failed to calculate device in use: %v", err)
		return err
	}
	klog.V(utils.INF).Infof("UpdateDeviceInUse: isUpdateCr: %v, newDeviceInUse: %v", isUpdateCr, newDeviceInUse)
	if isUpdateCr {
		// 3. update nodeStaticInfo
		err := UpdateVolumeDeviceMap(cm, needCreate, newDeviceInUse, hp)
		if err != nil {
			klog.Warningf("failed to update node volume device configmap: %v", err)
			return err
		}
	}
	return nil
}

func (hp *hostPath) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	klog.V(utils.INF).Info("now start to CreateVolume")
	if err := hp.validateControllerServiceRequest(csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME); err != nil {
		klog.Infof("the create volume request: %v is invalid", req)
		return nil, err
	}

	caps := req.GetVolumeCapabilities()
	if caps == nil {
		klog.Error("Volume Capabilities are missing in the request")
		return nil, status.Error(codes.InvalidArgument, "Volume Capabilities are missing in the request")
	}

	if len(req.GetName()) == 0 {
		klog.Error("Name is missing in the request")
		return nil, status.Error(codes.InvalidArgument, "Name is missing in the request")
	}

	// Keep a record of the requested access types.
	var atMount, atBlock bool

	for _, cap := range caps {
		if cap.GetBlock() != nil {
			atBlock = true
		}
		if cap.GetMount() != nil {
			atMount = true
		}
	}

	if atBlock && atMount {
		klog.Error("cannot have both block and mount access type")
		return nil, status.Error(codes.InvalidArgument, "cannot have both block and mount access type")
	}

	// get device id for volumeContext
	parameters := req.GetParameters()
	pvcName := parameters[PVCName]
	pvcNameSpace := parameters[PVCNameSpace]
	if pvcName == "" || pvcNameSpace == "" {
		klog.Errorf("pvcName(%s) or pvcNamespace(%s) can not be empty", pvcNameSpace, pvcName)
		return nil, status.Errorf(codes.InvalidArgument, "pvcName(%s) or pvcNamespace(%s) can not be empty", pvcNameSpace, pvcName)
	}
	cm, err := hp.coreClient.CoreV1().ConfigMaps(utils.IoiNamespace).Get(ctx, fmt.Sprintf("%s-%s", utils.DeviceIDConfigMap, hp.config.NodeID), metav1.GetOptions{})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "fail to get ConfigMap: %s due to %s", utils.DeviceIDConfigMap, err.Error())
	}
	deviceId := ""
	path := ""
	key := fmt.Sprintf("%s.%s", pvcName, pvcNameSpace)
	for k, v := range cm.Data {
		if k == key {
			strs := strings.Split(v, ":")
			if len(strs) == 2 {
				deviceId = strs[0]
				path = strs[1]
			}
			break
		}
	}
	if len(deviceId) == 0 {
		klog.Errorf("no mapping device id found for pvc %s.%s. Skip and wait for update.", pvcName, pvcNameSpace)
		return nil, status.Errorf(codes.InvalidArgument, "no mapping device id found for pvc %s.%s. Skip and wait for update.", pvcName, pvcNameSpace)
	}
	if len(path) == 0 {
		klog.Errorf("no mount path found for pvc %s.%s. Skip and wait for update.", pvcName, pvcNameSpace)
		return nil, status.Errorf(codes.InvalidArgument, "no mount path found for pvc %s.%s. Skip and wait for update.", pvcName, pvcNameSpace)
	}
	var requestedAccessType state.AccessType

	if atBlock {
		requestedAccessType = state.BlockAccess
	} else {
		// Default to mount.
		requestedAccessType = state.MountAccess
	}

	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.mutex.Lock()
	defer hp.mutex.Unlock()

	capacity := int64(req.GetCapacityRange().GetRequiredBytes())
	topologies := []*csi.Topology{}
	if hp.config.EnableTopology {
		topologies = append(topologies, &csi.Topology{Segments: map[string]string{TopologyKeyNode: hp.config.NodeID}})
	}

	// Need to check for already existing volume name, and if found
	if _, err := hp.state.GetVolumeByName(req.GetName()); err == nil {
		klog.Errorf("Volume with the same name: %s already exist", req.GetName())
		return nil, status.Errorf(codes.AlreadyExists, "Volume with the same name: %s already exist", req.GetName())
	}

	id, err := uuid.NewUUID()
	if err != nil {
		klog.Errorf("failed to created new uuid: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to created new uuid")
	}
	volumeID := id.String()
	path = filepath.Join(path, volumeID)
	kind := parameters[storageKind]
	parameters[utils.AnnoDeviceID] = deviceId
	parameters[utils.AnnoVolPath] = path

	vol, err := hp.createVolume(volumeID, deviceId, path, req.GetName(), capacity, requestedAccessType, kind)
	if err != nil {
		klog.Errorf("fail to create volume: %v", err)
		return nil, err
	}
	klog.V(utils.INF).Infof("created volume %s at path %s", vol.VolID, vol.VolPath)

	err = UpdateDeviceInUse(deviceId, true, hp)
	if err != nil {
		klog.Warningf("failed to update node static io info: %v", err)
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:           volumeID,
			CapacityBytes:      req.GetCapacityRange().GetRequiredBytes(),
			VolumeContext:      parameters,
			ContentSource:      req.GetVolumeContentSource(),
			AccessibleTopology: topologies,
		},
	}, nil
}

func (hp *hostPath) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	// Check arguments
	klog.V(utils.INF).Info("now start to DeleteVolume")
	if len(req.GetVolumeId()) == 0 {
		klog.Errorf("Volume ID missing in request: %v", req)
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	if err := hp.validateControllerServiceRequest(csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME); err != nil {
		klog.Errorf("invalid delete volume req: %v", req)
		return nil, err
	}
	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.mutex.Lock()
	defer hp.mutex.Unlock()

	volId := req.GetVolumeId()
	vol, err := hp.state.GetVolumeByID(volId)
	if err != nil {
		// Volume not found: might have already deleted
		klog.Errorf("failed to get volume %s: %v", volId, err)
		return &csi.DeleteVolumeResponse{}, nil
	}

	if vol.Attached || !vol.Published.Empty() || !vol.Staged.Empty() {
		msg := fmt.Sprintf("Volume '%s' is still used (attached: %v, staged: %v, published: %v) by '%s' node",
			vol.VolID, vol.Attached, vol.Staged, vol.Published, vol.NodeID)
		if hp.config.CheckVolumeLifecycle {
			klog.Errorf("DeleteVolume fail: volume %s is still used", volId)
			return nil, status.Error(codes.Internal, msg)
		}
		klog.Warning(msg)
	}

	if err := hp.deleteVolume(volId); err != nil {
		return nil, fmt.Errorf("failed to delete volume %v: %w", volId, err)
	}
	klog.V(utils.INF).Infof("volume %v successfully deleted", volId)

	err = UpdateDeviceInUse(vol.DeviceID, false, hp)
	if err != nil {
		klog.Warningf("failed to update node static io info: %v", err)
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func (hp *hostPath) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: hp.getControllerServiceCapabilities(),
	}, nil
}

func (hp *hostPath) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {

	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID cannot be empty")
	}
	if len(req.VolumeCapabilities) == 0 {
		return nil, status.Error(codes.InvalidArgument, req.VolumeId)
	}

	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.mutex.Lock()
	defer hp.mutex.Unlock()

	if _, err := hp.state.GetVolumeByID(req.GetVolumeId()); err != nil {
		return nil, err
	}

	for _, cap := range req.GetVolumeCapabilities() {
		if cap.GetMount() == nil && cap.GetBlock() == nil {
			return nil, status.Error(codes.InvalidArgument, "cannot have both mount and block access type be undefined")
		}

		// A real driver would check the capabilities of the given volume with
		// the set of requested capabilities.
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeContext:      req.GetVolumeContext(),
			VolumeCapabilities: req.GetVolumeCapabilities(),
			Parameters:         req.GetParameters(),
		},
	}, nil
}

func (hp *hostPath) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (hp *hostPath) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (hp *hostPath) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (hp *hostPath) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	volumeRes := &csi.ListVolumesResponse{
		Entries: []*csi.ListVolumesResponse_Entry{},
	}

	var (
		startIdx, volumesLength, maxLength int64
		hpVolume                           state.Volume
	)

	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.mutex.Lock()
	defer hp.mutex.Unlock()

	// Sort by volume ID.
	volumes := hp.state.GetVolumes()
	sort.Slice(volumes, func(i, j int) bool {
		return volumes[i].VolID < volumes[j].VolID
	})

	if req.StartingToken == "" {
		req.StartingToken = "1"
	}

	startIdx, err := strconv.ParseInt(req.StartingToken, 10, 32)
	if err != nil {
		return nil, status.Error(codes.Aborted, "The type of startingToken should be integer")
	}

	volumesLength = int64(len(volumes))
	maxLength = int64(req.MaxEntries)

	if maxLength > volumesLength || maxLength <= 0 {
		maxLength = volumesLength
	}

	for index := startIdx - 1; index < volumesLength && index < maxLength; index++ {
		hpVolume = volumes[index]
		healthy, msg := hp.doHealthCheckInControllerSide(hpVolume.VolID)
		klog.V(utils.DBG).Info("Healthy state: %s Volume: %t", hpVolume.VolName, healthy)
		volumeRes.Entries = append(volumeRes.Entries, &csi.ListVolumesResponse_Entry{
			Volume: &csi.Volume{
				VolumeId:      hpVolume.VolID,
				CapacityBytes: hpVolume.VolSize,
			},
			Status: &csi.ListVolumesResponse_VolumeStatus{
				PublishedNodeIds: []string{hpVolume.NodeID},
				VolumeCondition: &csi.VolumeCondition{
					Abnormal: !healthy,
					Message:  msg,
				},
			},
		})
	}

	klog.V(utils.DBG).Info("Volumes are: %+v", *volumeRes)
	return volumeRes, nil
}

func (hp *hostPath) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	// Lock before acting on global state. A production-quality
	// driver might use more fine-grained locking.
	hp.mutex.Lock()
	defer hp.mutex.Unlock()

	volume, err := hp.state.GetVolumeByID(req.GetVolumeId())
	if err != nil {
		// ControllerGetVolume should report abnormal volume condition if volume is not found
		return &csi.ControllerGetVolumeResponse{
			Volume: &csi.Volume{
				VolumeId: req.GetVolumeId(),
			},
			Status: &csi.ControllerGetVolumeResponse_VolumeStatus{
				VolumeCondition: &csi.VolumeCondition{
					Abnormal: true,
					Message:  err.Error(),
				},
			},
		}, nil
	}

	healthy, msg := hp.doHealthCheckInControllerSide(req.GetVolumeId())
	klog.V(utils.DBG).Info("Healthy state: %s Volume: %t", volume.VolName, healthy)
	return &csi.ControllerGetVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volume.VolID,
			CapacityBytes: volume.VolSize,
		},
		Status: &csi.ControllerGetVolumeResponse_VolumeStatus{
			PublishedNodeIds: []string{volume.NodeID},
			VolumeCondition: &csi.VolumeCondition{
				Abnormal: !healthy,
				Message:  msg,
			},
		},
	}, nil
}

// CreateSnapshot uses tar command to create snapshot for hostpath volume. The tar command can quickly create
// archives of entire directories. The host image must have "tar" binaries in /bin, /usr/sbin, or /usr/bin.
func (hp *hostPath) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (hp *hostPath) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (hp *hostPath) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (hp *hostPath) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (hp *hostPath) validateControllerServiceRequest(c csi.ControllerServiceCapability_RPC_Type) error {
	if c == csi.ControllerServiceCapability_RPC_UNKNOWN {
		return nil
	}

	for _, cap := range hp.getControllerServiceCapabilities() {
		if c == cap.GetRpc().GetType() {
			return nil
		}
	}
	return status.Errorf(codes.InvalidArgument, "unsupported capability %s", c)
}

func (hp *hostPath) getControllerServiceCapabilities() []*csi.ControllerServiceCapability {
	var cl []csi.ControllerServiceCapability_RPC_Type
	if !hp.config.Ephemeral {
		cl = []csi.ControllerServiceCapability_RPC_Type{
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
			// csi.ControllerServiceCapability_RPC_GET_VOLUME,
			csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
			csi.ControllerServiceCapability_RPC_VOLUME_CONDITION,
			csi.ControllerServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
		}
	}

	var csc []*csi.ControllerServiceCapability

	for _, cap := range cl {
		csc = append(csc, &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		})
	}

	return csc
}
