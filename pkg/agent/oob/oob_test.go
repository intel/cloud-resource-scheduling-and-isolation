/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package oob

import (
	"context"
	"github.com/agiledragon/gomonkey/v2"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/inotify"
	"os"
	"reflect"
	"sigs.k8s.io/IOIsolation/pkg/agent"
	"testing"
	"time"
)

func preparingCgroup(path string) {
	os.MkdirAll(path+"/cri-containerd-test0.scope", 0666)
}

func TestFindCgroupInfoInPod(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want []ContainerEvent
	}{
		{
			name: "Test1",
			args: args{path: "./kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod123.slice"},
			want: []ContainerEvent{
				{
					eventType:   0,
					podId:       "123",
					containerId: "test0",
					cgroupPath:  "./kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod123.slice",
					netns:       "",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preparingCgroup(tt.args.path)
			if got := FindCgroupInfoInPod(tt.args.path); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FindCgroupInfoInPod() = %v, want %v", got, tt.want)
			}
			os.RemoveAll(tt.args.path)
		})
	}
}

func TestParsePodId(t *testing.T) {
	type args struct {
		basename string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			"Test1",
			args{"kubepods-burstable-pod884758b4_0075_4bda_a0a9_174a8e82ec57.slice"},
			"884758b4_0075_4bda_a0a9_174a8e82ec57",
			false,
		},
		{
			"Test2",
			args{"pod884758b4_0075_4bda_a0a9_174a8e82ec57.slice"},
			"",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePodId(tt.args.basename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePodId() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParsePodId() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseContainerId(t *testing.T) {
	type args struct {
		basename string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			"Test1",
			args{"cri-containerd-262f36ab58014c804e0c901e2dcbce2644ddd955317ca9d2d462551f8dd17d47.scope"},
			"262f36ab58014c804e0c901e2dcbce2644ddd955317ca9d2d462551f8dd17d47",
			false,
		},
		{
			"Test2",
			args{"cri-262f36ab58014c804e0c901e2dcbce2644ddd955317ca9d2d462551f8dd17d47.scope"},
			"",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseContainerId(tt.args.basename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseContainerId() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseContainerId() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewOobServer(t *testing.T) {
	type args struct {
		cgroupRootPath string
	}
	tests := []struct {
		name    string
		args    args
		want    *OobServer
		wantErr bool
	}{
		{
			"Test 1",
			args{"/opt/ioi/cgroup/"},
			&OobServer{},
			false,
		},
		{
			"Test 2",
			args{""},
			&OobServer{},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOobServer(tt.args.cgroupRootPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOobServer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestOobServer_Initialize(t *testing.T) {
	pEnts := make(chan *PodEvent, 128)
	cEnts := make(chan *ContainerEvent, 128)
	pods2 := make(map[string]corev1.Pod)
	pods2["pod1"] = corev1.Pod{ObjectMeta: v1.ObjectMeta{UID: "123"}, Status: corev1.PodStatus{QOSClass: "Guaranteed"}}
	stub := &kubeletStub{}
	v2 := make(chan agent.EventData, 50)
	type fields struct {
		cgroupRootPath   string
		podWatcher       *inotify.Watcher
		containerWatcher *inotify.Watcher
		taskIdWatcher    *inotify.Watcher
		podEvents        chan *PodEvent
		containerEvents  chan *ContainerEvent
		kubeletStub      KubeletStub
		pods             map[string]corev1.Pod
		sentPodsCache    map[string]bool
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			"Test 1",
			fields{podWatcher: &inotify.Watcher{}, containerWatcher: &inotify.Watcher{}, taskIdWatcher: &inotify.Watcher{},
				podEvents: pEnts, containerEvents: cEnts, pods: pods2, kubeletStub: stub, cgroupRootPath: "/sys/fs/cgroup/"},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &OobServer{
				cgroupRootPath:   tt.fields.cgroupRootPath,
				podWatcher:       tt.fields.podWatcher,
				containerWatcher: tt.fields.containerWatcher,
				taskIdWatcher:    tt.fields.taskIdWatcher,
				podEvents:        tt.fields.podEvents,
				containerEvents:  tt.fields.containerEvents,
				kubeletStub:      tt.fields.kubeletStub,
				pods:             tt.fields.pods,
				sentPodsCache:    tt.fields.sentPodsCache,
			}

			op1 := []gomonkey.OutputCell{
				{Values: gomonkey.Params{corev1.PodList{}, nil}},
			}
			pt1 := gomonkey.ApplyMethodSeq(o.kubeletStub, "GetAllPods", op1)
			defer pt1.Reset()

			pt2 := gomonkey.ApplyGlobalVar(&agent.PodInfoChan, v2)
			defer pt2.Reset()

			if err := o.Initialize(); (err != nil) != tt.wantErr {
				t.Errorf("Initialize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOobServer_AnalysisOOBData(t *testing.T) {
	type fields struct {
		cgroupRootPath   string
		podWatcher       *inotify.Watcher
		containerWatcher *inotify.Watcher
		taskIdWatcher    *inotify.Watcher
		podEvents        chan *PodEvent
		containerEvents  chan *ContainerEvent
		kubeletStub      KubeletStub
		pods             map[string]corev1.Pod
		sentPodsCache    map[string]bool
	}
	type args struct {
		container *ContainerEvent
		netns     string
		op        int
	}
	sPC := make(map[string]bool)
	pds := make(map[string]corev1.Pod)

	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"Test 1",
			fields{sentPodsCache: sPC, pods: pds},
			args{&ContainerEvent{podId: "pod123", cgroupPath: "/sys/fs/cgroup/123"}, "netns-123", 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &OobServer{
				cgroupRootPath:   tt.fields.cgroupRootPath,
				podWatcher:       tt.fields.podWatcher,
				containerWatcher: tt.fields.containerWatcher,
				taskIdWatcher:    tt.fields.taskIdWatcher,
				podEvents:        tt.fields.podEvents,
				containerEvents:  tt.fields.containerEvents,
				kubeletStub:      tt.fields.kubeletStub,
				pods:             tt.fields.pods,
				sentPodsCache:    tt.fields.sentPodsCache,
			}
			o.AnalysisOOBData(tt.args.container, tt.args.netns, tt.args.op)
		})
	}
}

func TestGetNetNs(t *testing.T) {
	type args struct {
		ctx        context.Context
		cgroupPath string
	}
	ctx1, cancel1 := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel1()
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"Test 1",
			args{ctx: ctx1, cgroupPath: "/sys/fs/cgroup/123"},
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetNetNs(tt.args.ctx, tt.args.cgroupPath); got != tt.want {
				t.Errorf("GetNetNs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTypeOf(t *testing.T) {
	type args struct {
		event *inotify.Event
	}
	tests := []struct {
		name string
		args args
		want EventType
	}{
		{
			name: "Typeof DirCreated",
			args: args{event: &inotify.Event{
				Mask:   inotify.InCreate | inotify.InIsdir,
				Cookie: 0,
				Name:   "test",
			}},
			want: DirCreated,
		},
		{
			name: "Typeof DirRemoved",
			args: args{event: &inotify.Event{
				Mask:   inotify.InDelete | inotify.InIsdir,
				Cookie: 0,
				Name:   "test",
			}},
			want: DirRemoved,
		},
		{
			name: "Typeof FileCreated",
			args: args{event: &inotify.Event{
				Mask:   inotify.InCreate,
				Cookie: 0,
				Name:   "test",
			}},
			want: FileCreated,
		},
		{
			name: "Typeof Updated",
			args: args{event: &inotify.Event{
				Mask:   inotify.InModify,
				Cookie: 0,
				Name:   "test",
			}},
			want: FileUpdated,
		},
		{
			name: "Typeof Unkown",
			args: args{event: &inotify.Event{
				Mask:   0,
				Cookie: 0,
				Name:   "test",
			}},
			want: UnknownType,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TypeOf(tt.args.event); got != tt.want {
				t.Errorf("TypeOf() = %v, want %v", got, tt.want)
			}
		})
	}
}
