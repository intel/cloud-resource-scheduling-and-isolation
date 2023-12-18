/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package oob

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/nri/pkg/api"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/inotify"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

var cNetNsPath = "/var/run/netns"
var defaultNs = "default"
var NetNsTimeOut = 10

func init() {
	cNetNsPathStr := os.Getenv("CONTAINER_NETNS_PATH")
	if cNetNsPathStr != "" {
		cNetNsPath = cNetNsPathStr
	}
	defaultNsStr := os.Getenv("DEFAULT_NETNS_PATH")
	if defaultNsStr != "" {
		defaultNs = defaultNsStr
	}
}

type EventType int

const (
	InsecureKubeletTLS = false
	KubeletHttpsPort   = 10250
	KubeletHttpPort    = 10255
	HTTPScheme         = "http"
	HTTPSScheme        = "https"
	cgroupRootGAPath   = "kubepods.slice"
	cgroupRootBTPath   = "kubepods.slice/kubepods-burstable.slice"
	cgroupRootBEPath   = "kubepods.slice/kubepods-besteffort.slice"
	NonNRI             = "OOBServer(Non NRI)"

	DirCreated  = 0
	DirRemoved  = 1
	UnknownType = 2

	FileCreated = 0
	FileUpdated = 1

	PodAdded   = 0
	PodDeleted = 1

	ContainerAdded      = 0
	ContainerDeleted    = 1
	ContainerTaskIdDone = 2
)

type OobServer struct {
	cgroupRootPath   string
	podWatcher       *inotify.Watcher
	containerWatcher *inotify.Watcher
	taskIdWatcher    *inotify.Watcher
	podEvents        chan *PodEvent
	containerEvents  chan *ContainerEvent
	kubeletStub      KubeletStub
	pods             map[string]corev1.Pod
	sentPodsCache    map[string]bool
	Mutex            sync.Mutex
}

func (o *OobServer) Type() string {
	return NonNRI
}

func FindCgroupInfoInPod(path string) []ContainerEvent {
	containers := make([]ContainerEvent, 0, 10)
	files, err := os.ReadDir(path)
	if err != nil {
		klog.Errorf("Error reading folder:", err)
		return containers
	}

	for _, file := range files {
		if file.IsDir() && strings.HasPrefix(file.Name(), "cri-containerd") {
			fmt.Printf("Directory: %s\n", file.Name())
			containerId, err := ParseContainerId(filepath.Base(file.Name()))
			if err != nil {
				klog.Errorf("get containerId failed: %v", err)
				continue
			}
			podId, err := ParsePodId(filepath.Base(path))
			if err != nil {
				klog.Errorf("get podId failed, %v", err)
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(NetNsTimeOut)*time.Second)
			defer cancel()

			netns := GetNetNs(ctx, path+"/"+file.Name()+"/cgroup.procs")
			containerEvent := ContainerEvent{
				eventType:   0,
				podId:       podId,
				containerId: containerId,
				cgroupPath:  path,
				netns:       netns,
			}
			containers = append(containers, containerEvent)
			//file.Name() + "/cgroup.procs"
		}
	}
	return containers
}

func (o *OobServer) Initialize() error {
	klog.Info("3.3 non nri engine start")
	stopCtx := signals.SetupSignalHandler()
	go o.Run(stopCtx.Done())
	_, err := o.GetAllPods()
	if err != nil {
		klog.Errorf("Get all pods failed :%v", err)
	}

	go o.TimeGetAllPods(stopCtx.Done())

	dropRepeat := make(map[string]int)
	for _, pod := range o.pods {
		if _, ok := dropRepeat[string(pod.UID)]; ok {
			continue
		}
		dropRepeat[string(pod.UID)] = 1
		newPodId := strings.ReplaceAll(string(pod.UID), "-", "_")
		var cgroupPath string
		if pod.Status.QOSClass == "Guaranteed" {
			cgroupPath = o.cgroupRootPath + cgroupRootGAPath + "/kubepods-pod" + newPodId + ".slice"
		} else if pod.Status.QOSClass == "Burstable" {
			cgroupPath = o.cgroupRootPath + cgroupRootBTPath + "/kubepods-burstable-pod" + newPodId + ".slice"
		} else {
			cgroupPath = o.cgroupRootPath + cgroupRootBEPath + "/kubepods-besteffort-pod" + newPodId + ".slice"
		}

		containerEvents := FindCgroupInfoInPod(cgroupPath)
		for _, v := range containerEvents {
			o.AnalysisOOBData(&v, v.netns, utils.RunPod)
		}
	}
	return nil
}

func (o *OobServer) Uninitialize() error {
	return nil
}

type PodEvent struct {
	eventType  int
	podId      string
	cgroupPath string
}

type ContainerEvent struct {
	eventType   int
	podId       string
	containerId string
	cgroupPath  string
	netns       string
}

func NewOobServer(cgroupRootPath string) (*OobServer, error) {
	stub, err := newKubeletStub()
	if err != nil {
		klog.Errorf("%v", err)
	}
	podWatcher, err := inotify.NewWatcher()
	if err != nil {
		klog.Error("create pod watcher failed", err)
	}

	containerWatcher, err := inotify.NewWatcher()
	if err != nil {
		klog.Error("create container watcher failed", err)
	}

	taskIdWatcher, err := inotify.NewWatcher()
	if err != nil {
		klog.Error("create taskId watcher failed", err)
	}

	o := &OobServer{
		cgroupRootPath:   cgroupRootPath,
		podWatcher:       podWatcher,
		containerWatcher: containerWatcher,
		taskIdWatcher:    taskIdWatcher,
		kubeletStub:      stub,
		podEvents:        make(chan *PodEvent, 128),
		containerEvents:  make(chan *ContainerEvent, 128),
		pods:             make(map[string]corev1.Pod),
		sentPodsCache:    make(map[string]bool),
	}
	return o, nil
}

func newKubeletStub() (KubeletStub, error) {
	var scheme string
	var port int
	if InsecureKubeletTLS {
		scheme = HTTPScheme
		port = KubeletHttpPort
	} else {
		scheme = HTTPSScheme
		port = KubeletHttpsPort
	}
	nodeName := os.Getenv("Node_Name")
	return NewKubeletStub(nodeName, port, scheme, 30*time.Second)
}

// TypeOf tell the type of event
func TypeOf(event *inotify.Event) EventType {
	if event.Mask&inotify.InCreate != 0 && event.Mask&inotify.InIsdir != 0 {
		return DirCreated
	}
	if event.Mask&inotify.InDelete != 0 && event.Mask&inotify.InIsdir != 0 {
		return DirRemoved
	}
	if event.Mask&inotify.InCreate != 0 && event.Mask&inotify.InIsdir == 0 {
		return FileCreated
	}
	if event.Mask&inotify.InModify != 0 && event.Mask&inotify.InIsdir == 0 {
		return FileUpdated
	}
	return UnknownType
}

func (o *OobServer) AnalysisOOBData(container *ContainerEvent, netns string, op int) {
	newPodId := strings.ReplaceAll(container.podId, "_", "-")

	podCacheId := fmt.Sprintf("%s %d", newPodId, op)
	if op == utils.RunPod {
		if _, ok := o.sentPodsCache[podCacheId]; ok {
			return
		}
		o.sentPodsCache[podCacheId] = true
	} else {
		delete(o.sentPodsCache, podCacheId)
	}

	pod, ok := o.pods[newPodId]
	if !ok {
		klog.Errorf("can't find this pod %s", container.podId)
		return
	}
	netnamespace := make([]*api.LinuxNamespace, 1)
	if op == utils.RunPod {
		netnamespace[0] = &api.LinuxNamespace{
			Type: "network",
			Path: netns,
		}
	}

	cgroupPath := strings.ReplaceAll(container.cgroupPath, "/opt/ioi/cgroup", "")

	prf := agent.PodRunInfo{
		Operation:    op,
		PodName:      pod.Name,
		Namespace:    pod.Namespace,
		NetNamespace: netnamespace, // fixme: need to check later
		CGroupPath:   filepath.Dir(filepath.Dir(cgroupPath)),
	}
	if v, ok := pod.Annotations[utils.BlockIOConfigAnno]; ok {
		prf.DiskAnnotation = v
	}
	pif := agent.PodEventInfo{
		newPodId: prf,
	}

	PodInfoes := make(map[string]agent.PodEventInfo)
	PodInfoes["dev"] = pif
	pdata := agent.PodData{
		T:         0,
		PodEvents: PodInfoes,
	}
	if len(agent.PodInfoChan) == cap(agent.PodInfoChan) {
		klog.Error("PodInfoChan is full")
		return
	}
	agent.PodInfoChan <- &pdata
}

func GetNetNs(ctx context.Context, cgroupPath string) string {
	for {
		select {
		case <-ctx.Done():
			return ""
		default:
			file, err := os.Open(cgroupPath)
			if err != nil {
				klog.Errorf("no cgroup path now, %v %s", err, cgroupPath)
				return ""
			}
			defer file.Close()
			rd := bufio.NewReader(file)
			txt, _, err := rd.ReadLine()
			if err != nil {
				klog.Errorf("file %s readline failed %v", cgroupPath, err)
			}
			pid := string(txt)

			link, err := os.Readlink(fmt.Sprintf("/opt/ioi/proc/%s/ns/net", pid))
			if err != nil {
				klog.Errorf("Can't read link file %v:", err)
			}
			re := regexp.MustCompile(`\d+`)
			match := re.FindString(link)
			root := "/opt/ioi/run/netns/"
			var netns string
			err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					klog.Errorf("access path error: %v", err)
					return nil
				}

				if info.Mode()&os.ModeSymlink != 0 {
					return nil
				}

				str := strconv.FormatUint(info.Sys().(*syscall.Stat_t).Ino, 10)
				if str == match {
					klog.V(utils.INF).Infof("find path is %s, pid is %s", path, pid)
					if strings.EqualFold(filepath.Base(path), defaultNs) {
						return nil
					}
					netns = filepath.Join(cNetNsPath, filepath.Base(path))
					return filepath.SkipDir
				}
				return nil
			})

			if err != nil {
				klog.Errorf("Walk dir error: %v", err)
			}

			if netns != "" {
				klog.V(utils.INF).Infof("netns is %s", netns)
				return netns
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (o *OobServer) runEventHandler(stoptCh <-chan struct{}) {
	for {
		select {
		case event := <-o.podEvents:
			switch event.eventType {
			case PodAdded:
				klog.V(utils.INF).Infof("PodAdded, %s", event.podId)
				_, err := o.GetAllPods()
				if err != nil {
					klog.Errorf("Get all pods failed %v", err)
				}
			case PodDeleted:
				o.Mutex.Lock()
				delete(o.pods, event.podId)
				o.Mutex.Unlock()
				klog.V(utils.INF).Infof("PodDeleted, %s", event.podId)
			}
		case event := <-o.containerEvents:
			switch event.eventType {
			case ContainerAdded:
				klog.V(utils.INF).Infof("ContainerAdded, %s %s", event.podId, event.containerId)
			case ContainerDeleted:
				o.AnalysisOOBData(event, "", utils.StopPod)
				klog.V(utils.INF).Infof("ContainerDeleted, %s %s", event.podId, event.containerId)
			case ContainerTaskIdDone:
				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(NetNsTimeOut)*time.Second)
				defer cancel()
				netns := GetNetNs(ctx, event.cgroupPath)
				o.AnalysisOOBData(event, netns, utils.RunPod)
				klog.V(utils.INF).Infof("ContainerTaskIdDone, %s %s %s", event.podId, event.containerId, netns)
			}
		case <-stoptCh:
			return
		}
	}
}

func (o *OobServer) Run(stopCh <-chan struct{}) {
	cgroupGAPath := o.cgroupRootPath + cgroupRootGAPath
	err := o.podWatcher.AddWatch(cgroupGAPath, inotify.InCreate|inotify.InDelete)
	if err != nil {
		klog.Errorf("failed to watch path %s, err %v", cgroupGAPath, err)
		return
	}
	klog.V(utils.INF).Infof("add GAPath to watcher, %s", cgroupGAPath)
	cgroupBTPath := o.cgroupRootPath + cgroupRootBTPath
	err = o.podWatcher.AddWatch(cgroupBTPath, inotify.InCreate|inotify.InDelete)
	if err != nil {
		klog.Errorf("failed to watch path %s, err %v", cgroupBTPath, err)
		return
	}
	klog.V(utils.INF).Infof("add BTPath to watcher, %s", cgroupBTPath)
	cgroupBEPath := o.cgroupRootPath + cgroupRootBEPath
	err = o.podWatcher.AddWatch(cgroupBEPath, inotify.InCreate|inotify.InDelete)
	if err != nil {
		klog.Errorf("failed to watch path %s, err %v", cgroupBEPath, err)
		return
	}
	klog.V(utils.INF).Infof("add BEPath to watcher, %s", cgroupBEPath)
	defer func() {
		err := o.podWatcher.RemoveWatch(cgroupGAPath)
		if err != nil {
			klog.Errorf("failed to remove watch path %s, err %v", cgroupGAPath, err)
		}
		err = o.podWatcher.RemoveWatch(cgroupBTPath)
		if err != nil {
			klog.Errorf("failed to remove watch path %s, err %v", cgroupBTPath, err)
		}
		err = o.podWatcher.RemoveWatch(cgroupBEPath)
		if err != nil {
			klog.Errorf("failed to remove watch path %s, err %v", cgroupBEPath, err)
		}
	}()

	go o.runEventHandler(stopCh)
	for {
		select {
		case event := <-o.podWatcher.Event:
			switch TypeOf(event) {
			case DirCreated:
				podId, err := ParsePodId(filepath.Base(event.Name))
				if err != nil {
					klog.Errorf("failed to parse pod id from %s", event.Name)
				}
				err = o.containerWatcher.AddWatch(event.Name, inotify.InCreate|inotify.InDelete)
				if err != nil {
					klog.Errorf("failed to watch path %s, err %v", event.Name, err)
				}
				o.podEvents <- newPodEvent(podId, PodAdded, event.Name)
			case DirRemoved:
				podId, err := ParsePodId(filepath.Base(event.Name))
				if err != nil {
					klog.Errorf("failed to parse pod id from %s", event.Name)
				}
				err = o.containerWatcher.RemoveWatch(event.Name)
				if err != nil {
					klog.Errorf("failed to remove watch path %s, err %v", event.Name, err)
				}
				o.podEvents <- newPodEvent(podId, PodDeleted, event.Name)
				klog.V(utils.DBG).Infof("dir delete, %s", event.Name)
			default:
				klog.V(utils.DBG).Infof("Unkown type")
			}
		case err := <-o.podWatcher.Error:
			klog.Errorf("read pods event error: %v", err)
		case event := <-o.containerWatcher.Event:
			switch TypeOf(event) {
			case DirCreated:
				containerId, err := ParseContainerId(filepath.Base(event.Name))
				if err != nil {
					klog.Errorf("get containerId failed %v", err)
					continue
				}
				podId, err := ParsePodId(filepath.Base(filepath.Dir(event.Name)))
				if err != nil {
					klog.Errorf("get podId failed, %v", err)
					continue
				}
				err = o.taskIdWatcher.AddWatch(event.Name+"/cgroup.procs", inotify.InCreate|inotify.InModify|inotify.InAllEvents)
				if err != nil {
					klog.Errorf("failed to watch path %s, err %v", event.Name+"/cgroup.procs", err)
				}
				o.containerEvents <- newContainerEvent(podId, containerId, ContainerAdded, event.Name)
				klog.V(utils.DBG).Infof("dir create, %s", event.Name, podId, containerId)
			case DirRemoved:
				containerId, err := ParseContainerId(filepath.Base(event.Name))
				if err != nil {
					klog.Errorf("get containerId failed %v", err)
					continue
				}
				podId, err := ParsePodId(filepath.Base(filepath.Dir(event.Name)))
				if err != nil {
					klog.Errorf("get podId failed %v", err)
					continue
				}
				o.containerEvents <- newContainerEvent(podId, containerId, ContainerDeleted, event.Name)
				klog.V(utils.DBG).Infof("dir delete, %s", event.Name)
			default:
				klog.V(utils.DBG).Infof("Unkown type")
			}
		case event := <-o.taskIdWatcher.Event:
			switch TypeOf(event) {
			case FileCreated:
				klog.V(utils.DBG).Infof("cgroup.procs file created %v", event)

				containerDir := filepath.Dir(event.Name)
				containerId, err := ParseContainerId(filepath.Base(containerDir))
				if err != nil {
					klog.Errorf("get containerId failed %v", err)
					continue
				}
				podId, err := ParsePodId(filepath.Base(filepath.Dir(containerDir)))
				if err != nil {
					klog.Errorf("get podId failed, %v", err)
					continue
				}
				o.containerEvents <- newContainerEvent(podId, containerId, ContainerTaskIdDone, event.Name)
				klog.V(utils.DBG).Infof("dir create, %s", event.Name, podId, containerId)
			case FileUpdated:
				klog.V(utils.DBG).Infof("cgroup.procs file updated %v", event)

				containerDir := filepath.Dir(event.Name)
				containerId, err := ParseContainerId(filepath.Base(containerDir))
				if err != nil {
					klog.Errorf("get containerId failed %v", err)
					continue
				}
				podId, err := ParsePodId(filepath.Base(filepath.Dir(containerDir)))
				if err != nil {
					klog.Errorf("get podId failed, %v", err)
					continue
				}
				o.containerEvents <- newContainerEvent(podId, containerId, ContainerTaskIdDone, event.Name)
				err = o.taskIdWatcher.RemoveWatch(event.Name)
				if err != nil {
					klog.Errorf("failed to remove watch path %s, err %v", event.Name, err)
				}
				klog.V(utils.DBG).Infof("dir create, %s", event.Name, podId, containerId)
			}

		case <-stopCh:
			return
		}
	}
}

func ParsePodId(basename string) (string, error) {
	patterns := []struct {
		prefix string
		suffix string
	}{
		{
			prefix: "kubepods-besteffort-pod",
			suffix: ".slice",
		},
		{
			prefix: "kubepods-burstable-pod",
			suffix: ".slice",
		},

		{
			prefix: "kubepods-pod",
			suffix: ".slice",
		},
	}

	for i := range patterns {
		if strings.HasPrefix(basename, patterns[i].prefix) && strings.HasSuffix(basename, patterns[i].suffix) {
			return basename[len(patterns[i].prefix) : len(basename)-len(patterns[i].suffix)], nil
		}
	}
	return "", fmt.Errorf("fail to parse pod id: %v", basename)
}

func ParseContainerId(basename string) (string, error) {
	patterns := []struct {
		prefix string
		suffix string
	}{
		{
			prefix: "docker-",
			suffix: ".scope",
		},
		{
			prefix: "cri-containerd-",
			suffix: ".scope",
		},
	}

	for i := range patterns {
		if strings.HasPrefix(basename, patterns[i].prefix) && strings.HasSuffix(basename, patterns[i].suffix) {
			return basename[len(patterns[i].prefix) : len(basename)-len(patterns[i].suffix)], nil
		}
	}
	return "", fmt.Errorf("fail to parse container id: %v", basename)
}

func newPodEvent(podId string, eventType int, cgroupPath string) *PodEvent {
	return &PodEvent{eventType: eventType, podId: podId, cgroupPath: cgroupPath}
}

func newContainerEvent(podId string, containerId string, eventType int, cgroupPath string) *ContainerEvent {
	return &ContainerEvent{podId: podId, containerId: containerId, eventType: eventType, cgroupPath: cgroupPath}
}

func (o *OobServer) TimeGetAllPods(stopCh <-chan struct{}) {
	for {
		select {
		case <-time.After(time.Second * 10):
			_, err := o.GetAllPods()
			if err != nil {
				klog.Errorf("Get all pods failed %v", err)
			}
		case <-stopCh:
			return
		}
	}
}

func (o *OobServer) GetAllPods() (corev1.PodList, error) {
	if o.kubeletStub == nil {
		return corev1.PodList{}, fmt.Errorf("kubeletStub is nil")
	}
	pods, err := o.kubeletStub.GetAllPods()
	if err != nil {
		return pods, err
	}
	klog.V(utils.DBG).Infof("kubelet stub get %d pods", len(pods.Items))
	o.Mutex.Lock()
	for _, p := range pods.Items {
		o.pods[string(p.GetObjectMeta().GetUID())] = p
	}
	o.Mutex.Unlock()
	return pods, err
}

func (o *OobServer) IsPodRuning(podId string) bool {
	pod, ok := o.pods[podId]
	if !ok {
		return false
	} else {
		return pod.Status.Phase == corev1.PodRunning
	}
}
