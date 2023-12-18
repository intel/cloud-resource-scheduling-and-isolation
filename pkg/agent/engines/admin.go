/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package engines

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
	"sigs.k8s.io/IOIsolation/pkg/agent/common"
	pb "sigs.k8s.io/IOIsolation/pkg/api/ioiservice"
)

const (
	AdminName      = "admin-configmap"
	AdminNamespace = "ioi-system"
)

var (
	TypeAdmin = "Admin"
)

// AdminEngine
type AdminEngine struct {
	NodeName string
}

var eventMap map[interface{}]uint64

func (e *AdminEngine) Type() string {
	return TypeAdmin
}

func (e *AdminEngine) Initialize(coreClient *kubernetes.Clientset, client *versioned.Clientset, mtls bool) error {
	klog.Info("3.3 initializing the admin engine")
	eventMap = make(map[interface{}]uint64)
	go ServiceClientTHandler(agent.ClientHandlerChan)
	e.NodeName = os.Getenv("Node_Name")
	e.StartAdminConfig()

	return nil
}

func (e *AdminEngine) StartAdminConfig() {
	corefactory := informers.NewSharedInformerFactory(agent.GetAgent().CoreClient, 0)
	coreInformer := corefactory.Core().V1().ConfigMaps().Informer()

	handler := cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			adm, ok := obj.(*corev1.ConfigMap)
			if !ok || adm.Name != AdminName || adm.Namespace != AdminNamespace {
				return false
			}
			return true
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    e.AddAdminConfigData,
			UpdateFunc: e.UpdateAdminConfigData,
		},
	}
	if _, err := coreInformer.AddEventHandler(handler); err != nil {
		klog.Error("failed to bind the events to ConfigMaps")
		return
	}

	corefactory.Start(wait.NeverStop)
	corefactory.WaitForCacheSync(wait.NeverStop)
}

func (e *AdminEngine) AddAdminConfigData(obj interface{}) {
	ad, _ := obj.(*corev1.ConfigMap)
	data := e.GetAdminConfig(ad)
	data.InitFlag = true
	agent.AdminChan <- data
}

func (e *AdminEngine) UpdateAdminConfigData(oldObj, newObj interface{}) {
	ad, _ := newObj.(*corev1.ConfigMap)
	data := e.GetAdminConfig(ad)
	data.InitFlag = false
	agent.AdminChan <- data
}

func (e *AdminEngine) GetAdminConfig(ad *corev1.ConfigMap) *agent.AdminResourceConfig {
	var data agent.AdminResourceConfig
	data.NamespaceWhitelist = make(map[string]struct{})

	//disk pool
	data.DiskBePool = 20
	data.DiskSysPool = 20
	diskData, ok := ad.Data["diskpools"]
	if !ok {
		klog.Warning("diskpools not found")
	} else {
		split := strings.Split(string(diskData), "\n")
		splitLen := len(split) - 1
		for i := 0; i < splitLen; i++ {
			tmp := strings.Split(split[i], "=")
			value, err := strconv.Atoi(tmp[1])
			if err != nil {
				klog.Warning(err)
			} else {
				name := tmp[0]
				if name == "bePool" {
					data.DiskBePool = value
				} else if name == "sysPool" {
					data.DiskSysPool = value
				}
			}
		}
	}

	// net pool
	data.NetBePool = 20
	data.NetSysPool = 20
	netData, ok := ad.Data["networkpools"]
	if !ok {
		klog.Warning("networkpools not found")
	} else {
		split := strings.Split(string(netData), "\n")
		splitLen := len(split) - 1
		for i := 0; i < splitLen; i++ {
			tmp := strings.Split(split[i], "=")
			value, err := strconv.Atoi(tmp[1])
			if err != nil {
				klog.Warning(err)
			} else {
				name := tmp[0]
				if name == "bePool" {
					data.NetBePool = value
				} else if name == "sysPool" {
					data.NetSysPool = value
				}
			}

		}
	}

	// min pod bw
	data.Disk_min_pod_in_bw = 5
	data.Disk_min_pod_out_bw = 5
	data.Net_min_pod_in_bw = 5
	data.Net_min_pod_out_bw = 5
	minPodBwData, ok := ad.Data["min_pod_bw"]
	if !ok {
		klog.Warning("min_pod_bw not found")
	} else {
		split := strings.Split(string(minPodBwData), "\n")
		splitLen := len(split) - 1

		for i := 0; i < splitLen; i++ {
			tmp := strings.Split(split[i], "=")
			name := tmp[0]
			if len(tmp) < 2 {
				break
			}
			if name == "disk_read" {
				v, err := strconv.Atoi(tmp[1])
				if err != nil {
					klog.Warning(err)
				} else {
					if v <= 0 {
						klog.Warning("disk read value is not valid data")
					} else {
						data.Disk_min_pod_in_bw = v
					}
				}
			} else if name == "disk_write" {
				v, err := strconv.Atoi(tmp[1])
				if err != nil {
					klog.Warning(err)
				} else {
					if v <= 0 {
						klog.Warning("disk write value is not valid data")
					} else {
						data.Disk_min_pod_out_bw = v
					}
				}
			} else if name == "net_send" {
				v, err := strconv.Atoi(tmp[1])
				if err != nil {
					klog.Warning(err)
				} else {
					if v <= 0 {
						klog.Warning("net read value is not valid data")
					} else {
						data.Net_min_pod_in_bw = v
					}
				}
			} else if name == "net_receive" {
				v, err := strconv.Atoi(tmp[1])
				if err != nil {
					klog.Warning(err)
				} else {
					if v <= 0 {
						klog.Warning("net write value is not valid data")
					} else {
						data.Net_min_pod_out_bw = v
					}
				}
			}
		}
	}

	// log level
	loglevelData, ok := ad.Data["loglevel"]
	if !ok {
		klog.Warning("loglevel not found")
	} else {
		IsThisNode := false
		split := strings.Split(string(loglevelData), "\n")
		splitLen := len(split) - 1
		for i := 0; i < splitLen; i++ {
			tmp := strings.Split(split[i], "=")
			if len(tmp) < 2 {
				break
			}
			name := tmp[0]

			if name == "nodes" {
				re := regexp.MustCompile(tmp[1])
				sub := re.FindString(e.NodeName)
				if sub != "" {
					IsThisNode = true
				}
				break
			}
		}

		if IsThisNode {
			for i := 0; i < splitLen; i++ {
				tmp := strings.Split(split[i], "=")
				name := tmp[0]
				if len(tmp) < 2 {
					break
				}

				if name == "level" {
					if tmp[1] == "debug" {
						data.Loglevel.NodeAgentDebug = true
						data.Loglevel.AggregatorDebug = true
						data.Loglevel.ServiceDebug = true

						data.Loglevel.NodeAgentInfo = true
						data.Loglevel.AggregatorInfo = true
						data.Loglevel.ServiceInfo = true
					} else if tmp[1] == "info" {
						data.Loglevel.NodeAgentInfo = true
						data.Loglevel.AggregatorInfo = true
						data.Loglevel.ServiceInfo = true
					}
				}
			}
		}
		// set log level to manage the log
		utils.SetLogLevel(data.Loglevel.NodeAgentInfo, data.Loglevel.NodeAgentDebug)
		err := common.IOIConfigureLoglevel(utils.CommonIOIType, data.Loglevel.ServiceInfo, data.Loglevel.ServiceDebug)
		if err != nil {
			klog.Error(err)
		}
	}

	// interval
	data.Interval.NetInterval = 2
	data.Interval.DiskInterval = 2
	data.Interval.RdtInterval = 30

	intervalData, ok := ad.Data["reportInterval"]
	if !ok {
		klog.Warning("reportInterval not found")
	} else {
		split := strings.Split(string(intervalData), "\n")
		splitLen := len(split) - 1

		for i := 0; i < splitLen; i++ {
			tmp := strings.Split(split[i], "=")
			name := tmp[0]
			if len(tmp) < 2 {
				break
			}
			if name == "netInterval" {
				v, err := strconv.Atoi(tmp[1])
				if err != nil {
					klog.Warning(err)
				} else {
					data.Interval.NetInterval = v
				}
			} else if name == "diskInterval" {
				v, err := strconv.Atoi(tmp[1])
				if err != nil {
					klog.Warning(err)
				} else {
					data.Interval.DiskInterval = v
				}
			} else if name == "rdtInterval" {
				v, err := strconv.Atoi(tmp[1])
				if err != nil {
					klog.Warning(err)
				} else {
					data.Interval.RdtInterval = v
				}
			}
		}
	}

	// namespace whitelist
	data.NamespaceWhitelist["ioi-system"] = struct{}{}
	data.NamespaceWhitelist["kube-system"] = struct{}{}

	nsData, ok := ad.Data["namespaceWhitelist"]
	if !ok {
		klog.Warning("namespaceWhitelist not found")
	} else {
		ns := strings.Split(string(nsData), "\n")
		nsLen := len(ns) - 1
		for i := 0; i < nsLen; i++ {
			data.NamespaceWhitelist[ns[i]] = struct{}{}
		}
	}

	// system disk ratio
	data.SysDiskRatio = 50
	sysDiskRatio, ok := ad.Data["diskRatio"]
	if !ok {
		klog.Warning("diskRatio not found")
	} else {
		split := strings.Split(string(sysDiskRatio), "\n")
		splitLen := len(split) - 1
		for i := 0; i < splitLen; i++ {
			tmp := strings.Split(split[i], "=")
			if len(tmp) < 2 {
				break
			}
			v, err := strconv.Atoi(tmp[1])
			if err != nil {
				klog.Warning(err)
			} else {
				data.SysDiskRatio = v
			}
		}
	}

	// profile result times
	data.ProfileTime = 1
	profilerData, ok := ad.Data["profiler"]
	if !ok {
		klog.Warning("profiler not found")
	} else {
		split := strings.Split(string(profilerData), "\n")
		splitLen := len(split) - 1
		for i := 0; i < splitLen; i++ {
			tmp := strings.Split(split[i], "=")
			if len(tmp) < 2 {
				break
			}
			v, err := strconv.Atoi(tmp[1])
			if err != nil {
				klog.Warning(err)
			} else {
				data.ProfileTime = v
			}
		}
	}

	klog.V(utils.DBG).Infof("parsed Admin data is %+v", data)
	return &data
}

func ServiceClientTHandler(clientHandlerChan chan interface{}) {
	var err error
	for agent.IoiClient == nil {
		klog.Warning("ioi is nil, register app failed")
		time.Sleep(time.Second)
	}

	for event := range clientHandlerChan {
		switch r := event.(type) {
		case *pb.RegisterAppRequest:
			klog.V(utils.DBG).Info("RegisterApp, appId: ", r.AppName, " appInfo: ", r.AppInfo)
			_, err = agent.IoiClient.RegisterApp(agent.IoiCtx, r)
			if err != nil {
				klog.Warning("RegisterApp failed, appId: ", r.AppName, " error: ", err)
			}
		case *pb.UnRegisterAppRequest:
			klog.V(utils.DBG).Info("UnRegisterApp, appId: ", r.AppId)
			_, err = agent.IoiClient.UnRegisterApp(agent.IoiCtx, r)

		case *pb.SetAppLimitRequest:
			klog.V(utils.DBG).Info("SetAppLimit, appId: ", r.AppId, " limit: ", r.Limit)
			//_, err = ioi.SetAppLimit(ctx, r)
			podIds := strings.Split(r.AppId, ".")
			if len(podIds) >= 2 {
				podId := podIds[1]
				go RetrySetAppLimit(clientHandlerChan, agent.IoiClient, agent.IoiCtx, event, podId)
			} else { // for group app limit
				_, err = agent.IoiClient.SetAppLimit(agent.IoiCtx, r)
			}

		case *pb.ConfigureRequest:
			klog.V(utils.INF).Info("Config, appId: ", r.Type)
			_, err = agent.IoiClient.Configure(agent.IoiCtx, r)
		}

		if err != nil {
			klog.Warningf("Operation failed %s, will put it back channel ", err)
			switch status.Code(err) {
			case codes.Unavailable:
				time.Sleep(time.Second / 2)
				clientHandlerChan <- event
			}
		}
	}
}

func RetrySetAppLimit(clientHandlerChan chan interface{}, ioi pb.IoiserviceClient, ctx context.Context, event interface{}, podUid string) {
	var err error

	if IsPodRuning(podUid) {
		r := event.(*pb.SetAppLimitRequest)
		_, err = ioi.SetAppLimit(ctx, r)
		if err != nil {
			klog.Warningf("event failed %s", err)
			time.Sleep(time.Second / 2)
			clientHandlerChan <- event
		} else {
			delete(eventMap, event)
		}
	} else {
		if _, ok := eventMap[event]; !ok {
			eventMap[event] = 1
		} else {
			eventMap[event] += 1
		}

		times := eventMap[event]
		if times < 20 {
			time.Sleep(time.Duration(times*3) * time.Second)
		} else {
			time.Sleep(60 * time.Second)
		}
		klog.Warningf("Pod %s is not running, skip setAppLimit", podUid)

		clientHandlerChan <- event
	}
}

// Uninitialize
func (e *AdminEngine) Uninitialize() error {
	return nil
}

func init() {
	engine := &AdminEngine{}
	agent.GetAgent().RegisterEngine(engine)
}
