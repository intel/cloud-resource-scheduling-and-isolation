/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package blockio

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	utils "sigs.k8s.io/IOIsolation/pkg"
	pb "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	ioiresource "sigs.k8s.io/IOIsolation/pkg/scheduler/resource"
	testutils "sigs.k8s.io/IOIsolation/test/utils"
)

func TestResource_AddPod(t *testing.T) {
	type fields struct {
		nodeName string
		info     *NodeInfo
		ch       ioiresource.CacheHandle
		// RWMutex  sync.RWMutex
	}
	type args struct {
		pod        *corev1.Pod
		rawRequest interface{}
		client     kubernetes.Interface
	}
	scName := "sc1"
	ctx := context.Background()
	fakeKubeClient := fakekubeclientset.NewSimpleClientset()
	if err := testutils.MakeVolumeDeviceMapConfigMap(fakeKubeClient, ctx, "node1", map[string]string{}); err != nil {
		t.Error(err)
	}
	if err := testutils.MakeStorageClass(fakeKubeClient, ctx, scName, "localstorage.csi.k8s.io"); err != nil {
		t.Error(err)
	}
	if err := testutils.MakePVC(fakeKubeClient, ctx, &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc1",
			Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
		},
	}); err != nil {
		t.Error(err)
	}
	time.Sleep(time.Second)
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []*pb.IOResourceRequest
		want1   bool
		wantErr bool
	}{
		{
			name: "add pvc attached pod",
			fields: fields{
				nodeName: "node1",
				info: &NodeInfo{
					DisksStatus: map[string]*DiskInfo{
						"dev1": {
							GA: &utils.IOPoolStatus{
								Total: 100,
								In:    50,
								Out:   50,
							},
							BE: &utils.IOPoolStatus{
								Num: 1,
							},
						},
					},
				},
				ch: &Handle{
					HandleBase: ioiresource.HandleBase{
						EC: fakeResourceCache(&utils.IOPoolStatus{}, &utils.IOPoolStatus{}, 0),
					},
				},
			},
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "default",
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "vol1",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "pvc1",
									},
								},
							},
						},
					},
				},
				rawRequest: &RequestInfo{
					ResourceRequest: map[string]*utils.IOResourceRequest{
						"dev1": {
							IORequest: &pb.IOResourceRequest{
								IoType:       pb.IOType_DiskIO,
								WorkloadType: utils.GaIndex,
								DevRequest: &pb.DeviceRequirement{
									DevName: "dev1",
									Request: &pb.Quantity{
										Inbps:  10,
										Outbps: 10,
									},
									Limit: &pb.Quantity{
										Inbps:  10,
										Outbps: 10,
									},
								},
							},
						},
					},
					PvcToDevice: map[string]*utils.PVCInfo{
						"pvc1.default": {
							DevID:      "dev1",
							ReqStorage: 10,
							Reuse:      false,
						},
					},
				},
				client: fakeKubeClient,
			},
			want: []*pb.IOResourceRequest{
				{
					IoType:       pb.IOType_DiskIO,
					WorkloadType: utils.GaIndex,
					DevRequest: &pb.DeviceRequirement{
						DevName: "dev1",
						Request: &pb.Quantity{
							Inbps:  10,
							Outbps: 10,
						},
						Limit: &pb.Quantity{
							Inbps:  10,
							Outbps: 10,
						},
					},
				},
			},
			want1:   true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		ioiresource.IoiContext = &ioiresource.ResourceIOContext{
			ClaimDevMap: map[string]*ioiresource.StorageInfo{},
		}
		t.Run(tt.name, func(t *testing.T) {
			ps := &Resource{
				nodeName: tt.fields.nodeName,
				info:     tt.fields.info,
				ch:       tt.fields.ch,
				// RWMutex:  tt.fields.RWMutex,
			}
			got, got1, err := ps.AddPod(tt.args.pod, tt.args.rawRequest, tt.args.client)
			if (err != nil) != tt.wantErr {
				t.Errorf("Resource.AddPod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Resource.AddPod() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Resource.AddPod() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestResource_AdmitPod(t *testing.T) {
	type fields struct {
		nodeName string
		info     *NodeInfo
		ch       ioiresource.CacheHandle
		// RWMutex  sync.RWMutex
	}
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    int64
		wantErr bool
	}{
		{
			name: "Admit new pod",
			fields: fields{
				nodeName: "node1",
				info: &NodeInfo{
					DisksStatus: map[string]*DiskInfo{
						"dev1": {
							GA: &utils.IOPoolStatus{
								Total: 100,
								In:    50,
								Out:   50,
							},
							BE: &utils.IOPoolStatus{
								Num: 1,
							},
						},
					},
				},
				ch: &Handle{
					HandleBase: ioiresource.HandleBase{
						EC: fakeResourceCache(&utils.IOPoolStatus{}, &utils.IOPoolStatus{}, 0),
					},
				},
			},
			args: args{
				pod: &corev1.Pod{},
			},
			want:    10,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := &Resource{
				nodeName: tt.fields.nodeName,
				info:     tt.fields.info,
				ch:       tt.fields.ch,
				// RWMutex:  tt.fields.RWMutex,
			}
			h := &Handle{}
			patches := gomonkey.ApplyMethod(reflect.TypeOf(h), "GetRequestByAnnotation", func(_ *Handle, pod *corev1.Pod, s string, ok bool) (map[string]*pb.IOResourceRequest, interface{}, error) {
				return map[string]*pb.IOResourceRequest{
						"dev1": {
							IoType:       pb.IOType_DiskIO,
							WorkloadType: utils.GaIndex,
							DevRequest: &pb.DeviceRequirement{
								DevName: "dev1",
								Request: &pb.Quantity{
									Inbps:  100,
									Outbps: 100,
								},
								Limit: &pb.Quantity{
									Inbps:  100,
									Outbps: 100,
								},
							},
						},
					}, map[string]*utils.PVCInfo{
						"pvc1.default": {
							DevID:      "dev1",
							ReqStorage: 10,
							Reuse:      false,
						},
					}, nil
			})

			defer patches.Reset()
			got, err := ps.AdmitPod(tt.args.pod)
			if (err != nil) != tt.wantErr {
				t.Errorf("Resource.AdmitPod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			st := got.(*RequestInfo).ResourceRequest["dev1"].StorageRequest
			if !reflect.DeepEqual(st, tt.want) {
				t.Errorf("Resource.AdmitPod() = %v, want %v", st, tt.want)
			}
		})
	}
}

func TestResource_RemovePod(t *testing.T) {
	type fields struct {
		nodeName string
		info     *NodeInfo
		ch       ioiresource.CacheHandle
		// RWMutex  sync.RWMutex
	}
	type args struct {
		pod    *corev1.Pod
		client kubernetes.Interface
	}
	scName := "sc1"
	ctx := context.Background()
	mSuccess := map[string]string{
		fmt.Sprintf("%s.%s", "pvc1", "default"): "dev1",
	}
	fakeKubeClient := fakekubeclientset.NewSimpleClientset()
	if err := testutils.MakeVolumeDeviceMapConfigMap(fakeKubeClient, ctx, "node1", mSuccess); err != nil {
		t.Error(err)
	}
	if err := testutils.MakeStorageClass(fakeKubeClient, ctx, scName, "localstorage.csi.k8s.io"); err != nil {
		t.Error(err)
	}
	if err := testutils.MakePVC(fakeKubeClient, ctx, &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc1",
			Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
		},
	}); err != nil {
		t.Error(err)
	}
	time.Sleep(time.Second)
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "remove pod with pvc",
			fields: fields{
				nodeName: "node1",
				info: &NodeInfo{
					DisksStatus: map[string]*DiskInfo{
						"dev1": {
							GA: &utils.IOPoolStatus{
								Total: 100,
								In:    50,
								Out:   50,
							},
							BE: &utils.IOPoolStatus{
								Num: 1,
							},
						},
					},
				},
				ch: &Handle{
					HandleBase: ioiresource.HandleBase{
						EC: fakeResourceCache(&utils.IOPoolStatus{}, &utils.IOPoolStatus{}, 0),
					},
				},
			},
			args: args{
				client: fakeKubeClient,
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						UID:       "podid1",
						Name:      "pod1",
						Namespace: "default",
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "vol1",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "pvc1",
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		ioiresource.IoiContext = &ioiresource.ResourceIOContext{
			ClaimDevMap: map[string]*ioiresource.StorageInfo{
				"pvc1.default": {
					DevID:            "dev1",
					RequestedStorage: 10,
					NodeName:         "node1",
				},
			},
			Reservedpod: map[string]*pb.PodList{
				"node1": {
					PodList: map[string]*pb.PodRequest{
						"podid1": {
							PodName: "pod1",
							Request: []*pb.IOResourceRequest{
								{
									IoType:       pb.IOType_DiskIO,
									WorkloadType: utils.GaIndex,
									DevRequest: &pb.DeviceRequirement{
										DevName: "dev1",
										Request: &pb.Quantity{
											Inbps:  50,
											Outbps: 50,
										},
									},
								},
							},
						},
					},
				},
			},
		}
		t.Run(tt.name, func(t *testing.T) {
			ps := &Resource{
				nodeName: tt.fields.nodeName,
				info:     tt.fields.info,
				ch:       tt.fields.ch,
				// RWMutex:  tt.fields.RWMutex,
			}
			if err := ps.RemovePod(tt.args.pod, tt.args.client); (err != nil) != tt.wantErr {
				t.Errorf("Resource.RemovePod() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
