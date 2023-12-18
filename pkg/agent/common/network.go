/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package common

import (
	"encoding/json"
	"os"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
	pb "sigs.k8s.io/IOIsolation/pkg/api/ioiservice"
)

const (
	AverageTimeNet int = 1
	NICMAX         int = 1000
)

var MinPodNetInBw float64 = 5.0
var MinPodNetOutBw float64 = 5.0

var NodeIPs []NodeIP // from profile

type NetAppInfo struct {
	Group      string            `json:"group"`
	NetNs      string            `json:"netns"`
	Devices    map[string]string `json:"devices"`
	CgroupPath string            `json:"cgroupPath"`
}
type Group struct {
	Id     string `json:"id"`
	Types  int    `json:"groupType"`
	Pool   int    `json:"pool"`
	Queues int    `json:"queues"`
}

type NodeIP struct {
	InterfaceName string `json:"interfacename"`
	IpAddr        string `json:"ipaddr"`
	MacAddr       string `json:"macaddr"`
	InterfaceType string `json:"interfacetype"`
}
type ConfigInfo struct {
	Cni       string   `json:"cni"`
	GatewayIp string   `json:"gateway"`
	NodeIps   []NodeIP `json:"nodeips"`
	Groups    []Group  `json:"groups"`
}

type GroupSettableInfo struct {
	SettableInfo []SettableInfo `json:"groups"`
}

type SettableInfo struct {
	Id            string `json:"id"`
	GroupSettable bool   `json:"group_settable"`
}

func CheckGRPCStatus() (bool, error) {
	for {
		if agent.IoiClient == nil {
			klog.Error("ioiClient is nil, CheckGRPCStatus failed")
			time.Sleep(time.Second * 1)
		} else {
			break
		}
	}

	return true, nil
}

func IOIGetServiceInfoNet() *agent.AllProfile {
	var allProfile agent.AllProfile
	var nicProfile utils.NicInfo
	var hostInfo HostInfo

	hostInfo.IpAddr = os.Getenv("Node_IP")
	data, json_err := json.Marshal(hostInfo)
	if json_err != nil {
		klog.Warningf("Marshal is failed")
		return nil
	}
	klog.V(utils.INF).Info("string(data): ", string(data))

	ServiceInfoResponse, err := agent.IoiClient.GetServiceInfo(agent.IoiCtx, &pb.ServiceInfoRequest{IoiType: utils.NetIOIType, Req: string(data)})
	if err != nil {
		klog.Warning("GetServiceInfo failed, please wait for server to start:", err)
		return nil
	} else {
		klog.V(utils.INF).Infof("GetServiceInfo success GetServiceInfo %v\n", ServiceInfoResponse.Configuration)
		err = json.Unmarshal([]byte(ServiceInfoResponse.Configuration), &nicProfile)
		if err == nil {
			klog.V(utils.INF).Infof("parse ServiceInfoResponse.Configuration success, nicInfo: %v\n", nicProfile)
			nicProfile.Type = "masterNic"
			nicProfile.IpAddr = hostInfo.IpAddr

			profileConfig := make(map[string]utils.NicInfo)
			profileConfig[nicProfile.Macaddr] = nicProfile
			allProfile.T = 1
			allProfile.NProfile = profileConfig

		} else {
			klog.Errorf("parse ServiceInfoResponse failed because string format is wrong, %s\n", ServiceInfoResponse.Configuration)
			return nil
		}
		return &allProfile
	}
}

func IOISetLimitNet(podId string, bw *ComposeBWLimit, ninfo *utils.NicInfo) {
	var bws []*pb.BWLimit
	var lmt pb.BWLimit

	lmt.Id = "0" // only one nic now

	if bw.Wl.InBps == LimitMax {
		lmt.In = strconv.Itoa(int(NICMAX))
	} else {
		lmt.In = strconv.Itoa(int(bw.Wl.InBps))
	}

	if bw.Wl.OutBps == LimitMax {
		lmt.Out = strconv.Itoa(int(NICMAX))
	} else {
		lmt.Out = strconv.Itoa(int(bw.Wl.OutBps))
	}

	bws = append(bws, &lmt)
	klog.V(utils.DBG).Infof("SetAppLimit id: %s, In:%s, Out:%s", lmt.Id, lmt.In, lmt.Out)

	agent.ClientHandlerChan <- &pb.SetAppLimitRequest{AppId: podId, Limit: bws, IoiType: utils.NetIOIType}
}

func IOIRegisterNet(podUid string, group string, netNs string, nodeIps []NodeIP, cgroupPath string) {
	var netInfo NetAppInfo
	netInfo.Group = group
	netInfo.NetNs = netNs
	netInfo.CgroupPath = cgroupPath
	netInfo.Devices = make(map[string]string)
	// get devices from profile
	NodeIPs = nodeIps
	for _, ip := range NodeIPs {
		netInfo.Devices["eth0"] = ip.IpAddr
	}
	data, json_err := json.Marshal(netInfo)
	if json_err != nil {
		klog.Warningf("marshal is failed")
	}
	agent.ClientHandlerChan <- &pb.RegisterAppRequest{AppName: podUid, AppInfo: string(data), IoiType: utils.NetIOIType}
}

func IOIUnRegisterNet(podUid string) {
	agent.ClientHandlerChan <- &pb.UnRegisterAppRequest{AppId: podUid, IoiType: utils.NetIOIType}
}

func IOIConfigureNet(cni string, gatewayIp string, nodeIps []NodeIP, netGroups []Group) GroupSettableInfo {
	var s GroupSettableInfo
	var configInfo ConfigInfo
	configInfo.Cni = cni
	configInfo.GatewayIp = gatewayIp
	configInfo.NodeIps = nodeIps
	configInfo.Groups = netGroups
	data, jsonErr := json.Marshal(configInfo)
	if jsonErr != nil {
		klog.Error("Marshal is failed: ", jsonErr)
	}
	klog.V(utils.DBG).Infof("configInfo is %v, data is %v", configInfo, data)
	if agent.IoiClient != nil {
		info, err := agent.IoiClient.Configure(agent.IoiCtx, &pb.ConfigureRequest{Configure: string(data), IoiType: utils.NetIOIType})
		if err != nil {
			klog.Error("Configure net failed:", err)
		}
		err = json.Unmarshal([]byte(info.ResponseInfo), &s)
		if err != nil {
			klog.Error("parse group settable failed", err)
		}
	} else {
		klog.Error("ioiClient is nil, configure network failed")
	}

	return s
}

func IOISubscribeNet(NicInfos map[string]utils.NicInfo) {
	resultArr := make([]agent.PodData, AverageTimeNet)
	count := 0
	index := 0

	for {
		stream, err := agent.IoiClient.Subscribe(agent.IoiCtx, &pb.SubscribeContext{
			IoiType: utils.NetIOIType,
		}, grpc.WaitForReady(true))
		if err != nil {
			klog.Warning("Subscribe error: ", err)
		} else {
			klog.V(utils.DBG).Info("Subscribe succeed")
		}

		for {
			resp, err := stream.Recv()
			if err != nil {
				klog.Warning("Waiting for subscribe restart, Stream recv failed error: ", err)
				break
			}

			// update data
			result := parseReceiveMessageNet(resp, NicInfos)
			count++
			index = (count - 1) % AverageTimeNet
			resultArr[index] = *result
			klog.V(utils.DBG).Info("Net subscribed pods data from service: ", *result)
			// in this version, getAverageResult returns directly the request ingress/egress

			averageResult := getAverageResult(resultArr)
			klog.V(utils.DBG).Info("Net subscribed pods data average: ", averageResult)
			agent.PodInfoChan <- &averageResult

			// we will return count to 10 after one day
			if count == AverageTimeNet*8640 {
				count = AverageTimeNet
			}
		}
	}
}

func parseReceiveMessageNet(resp *pb.AppsBandwidth, NicInfos map[string]utils.NicInfo) *agent.PodData {
	var result agent.PodData
	podEvents := make(map[string]agent.PodEventInfo)

	appBWInfo := resp.GetNetworkIoBw().AppBwInfo
	for po, bwInfo := range appBWInfo {
		var nicId string
		for _, veth := range bwInfo.BwInfo {
			var wl utils.Workload
			val, err := strconv.ParseFloat(veth.Value, 64)
			if err != nil {
				klog.Error("failed to parse veth.Value into float, the value is: ", veth.Value)
			} else {

				if veth.IsIngress {
					wl.InBps = val / utils.MB * 8 // Mbit/s
				} else {
					wl.OutBps = val / utils.MB * 8 // Mbit/s
				}
			}

			podRunInfo := agent.PodRunInfo{
				Actual: &wl,
			}

			nicId = ""
			for id := range NicInfos {
				nicId = id
				break
			}
			if nicId == "" {
				klog.Warning("no such nic found")
				continue
			}

			if podEvents[nicId] == nil {
				podEvents[nicId] = make(map[string]agent.PodRunInfo)
			}
			podEvents[nicId][po] = podRunInfo
		}
	}

	result.T = 3
	result.PodEvents = podEvents
	return &result
}
