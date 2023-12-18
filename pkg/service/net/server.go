/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package net

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	utils "sigs.k8s.io/IOIsolation/pkg"

	"github.com/safchain/ethtool"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
	pb "sigs.k8s.io/IOIsolation/pkg/api/ioiservice"
	"sigs.k8s.io/IOIsolation/pkg/service"
	"sigs.k8s.io/IOIsolation/pkg/service/common"
)

const (
	OP_ADD         int32  = 1
	OP_DEL         int32  = 0
	MacPath        string = "/sys/class/net/%s/address"
	SpeedPath      string = "/sys/class/net/%s/speed"
	Pollers        uint64 = 1
	Flannel        string = "flannel"
	Calico         string = "calico"
	Cilium         string = "cilium-cni"
	Bridge         string = "bridge"
	DefaultSpeed   string = "1000"
	Eth0           string = "eth0"
	DefaultMacAddr string = "0:0:0:0:0:0"
)

var (
	NetworkConfig ServiceInfoConfiguration
	BridgeMode    = false
)

type RegisterAppInfo struct {
	Group      string            // which group need to be registered by this app
	NetNS      string            // app's network namespace
	Devices    map[string]string // pod's nic => ip
	CgroupPath string
}

type Nic struct {
	Name  string // NIC name
	Ip    string // NIC IP
	IsAdq bool   // support Adq or not
}

type SR struct {
	Send    float64
	Receive float64
}

type Veth struct {
	HostVeth string
	Veth     string
	Traffic  SR
}
type GroupSettable struct {
	Id       string `json:"id"`
	Settable bool   `json:"group_settable"`
}

type ConfigureRespInfo struct {
	Groups []GroupSettable `json:"groups"`
}

type BWLimit struct {
	Id      string // key of devices
	Ingress string // ingress
	Egress  string // egress
}

type AppInfo struct {
	AppName string `json:"appname"`
	// app belongs to which group
	Group string `json:"group"`
	// network namespace
	NetNS string `json:"netns"`
	// cgroup path
	CgroupPath string `json:"cgroup_path"`
	// pod's nic => ip
	Devices map[string]string `json:"devices"`
	// pod's limit
	Limits []BWLimit `json:"limits"`
	// ifb name
	IfbName string `json:"ifb_name"`
}

// Group has two types now:
// 0. lazy limit the app in the group separately => GROUP_SEPARATE_LAZY
// 1. all the app shares one group control, only need to control the group => GROUP_SHARE
type Group struct {
	Id        string // group id
	GroupType int    // indicate that whether this group need to be controlled at group level or app level
	Pool      uint64 // group pool size
	Queues    uint64 // queue number of nic
}

type NICInfo struct {
	InterfaceName string
	IpAddr        string
	MacAddr       string
	InterfaceType string
}

type CNI struct {
	Name    string // cni name
	Gateway string // gateway ip
}

type ServiceInfo struct {
	IP string `json:"ip"`
}
type ServiceInfoConfiguration struct {
	MacAddr       string `json:"mac_addr"`
	InterfaceName string `json:"interface_name"`
	DefaultSpeed  uint64 `json:"default_speed"`
	QueueNum      uint64 `json:"queue_num"`
	CNI           string `json:"cni"`
	Backend       string `json:"backend"`
}
type ConfigureInfo struct {
	Cni     string    // which cni system use, such as calico
	Gateway string    // Gateway ip of cni
	NodeIps []NICInfo // host nic which main cni use
	Groups  []Group   // all the groups needed to be configured
}

type ServerInfo struct {
	ServeTime  string
	Generation uint64
}

var (
	ADQ    = "adq"
	NonAdq = "non-adq"
)

type Option struct {
	EnableSubscribe bool
	IsBridgeCNI     bool
	Speed           uint64
	Ifname          string
}

type NetEngine struct {
	service.ServiceEngine
	persist       common.IPersist
	si            ServerInfo // serverInfo
	option        Option
	groupSettable string
	Apps          map[string]*AppInfo // app_id => app_info
	groups        []Group
	groupLimit    map[string]BWLimit       // group_id -> BWLimit
	nics          []NICInfo                // all the host network adapter ip
	cnis          []CNI                    // multiple cni in the future, now use cnis[0] as main cnil
	Interval      utils.IntervalConfigInfo // current configuration interval
	plugins       map[string]Plugin        // host network_ip => plugins
	prevMonitor   map[string]SR
	podsVethCache map[string][]Veth
}

var mutex sync.Mutex

func (e *NetEngine) Type() int32 {
	return utils.NetIOIType
}

func (n *NetEngine) Initialize(persist common.IPersist) error {
	// create buckets
	n.persist = persist
	netBuckets := []string{"APPS", "GROUPS", "GROUPLIMIT", "NICS", "CNIS"}
	err := persist.CreateTable("NET", netBuckets)
	if err != nil {
		klog.Warningf("Create net buckets fail: %s", err)
	}

	n.option = Option{
		EnableSubscribe: viper.GetBool("network.enable_subscribe"),
		IsBridgeCNI:     viper.GetBool("network.is_bridge_cni"),
		Speed:           viper.GetUint64("network.speed"),
		Ifname:          viper.GetString("network.ifname"),
	}

	n.nics = make([]NICInfo, 0)
	n.groups = make([]Group, 0)
	n.groupLimit = make(map[string]BWLimit)
	n.plugins = make(map[string]Plugin)
	n.cnis = make([]CNI, 1)
	n.si = ServerInfo{}
	n.prevMonitor = make(map[string]SR)
	n.podsVethCache = make(map[string][]Veth)
	n.Apps = make(map[string]*AppInfo)
	// load app info from db
	apps, err1 := GetAllAppInfos(n, "")
	if err1 != nil {
		klog.Warningf("Get all app infos fail: %s", err)
	} else {
		n.Apps = apps
	}

	// for interval
	n.Interval = utils.IntervalConfigInfo{
		Interval: 2,
	}
	configInterval, err1 := LoadInterval(n)
	if err1 != nil {
		klog.Warningf("Get interval failed: ", err1)
	} else {
		n.Interval = configInterval
	}

	BridgeMode = n.option.IsBridgeCNI

	clearPodTme := viper.GetInt64("disk.clear_pod_time")
	go n.CheckPodCgroup(clearPodTme)
	// todo: start monitor
	si, err := GetServerInfo(n)
	if err != nil {
		klog.Errorf("get server %v info %v.", si, err)
		si = ServerInfo{
			ServeTime:  time.Now().Format(time.UnixDate),
			Generation: 0,
		}
	} else {
		klog.V(utils.DBG).Info("persistent GetServerInfo: ", si)
	}

	if si.Generation == 0 {
		klog.V(utils.DBG).Info("io service first run")
	} else if err = n.rebuild(); err != nil {
		klog.Errorf("failed to restore data  %v.", err)
	}

	si = ServerInfo{
		ServeTime:  time.Now().Format(time.UnixDate),
		Generation: si.Generation + 1,
	}
	err = AddServerInfo(n, si)
	if err != nil {
		klog.Errorf("persistent addServerInfo %v failed, and the reason is: %v", si, err)
	} else {
		klog.V(utils.DBG).Info("persistent addServerInfo: %v", si)
	}

	return nil
}

func (n *NetEngine) Uninitialize() error {
	return nil
}

func (n *NetEngine) CheckPodCgroup(clearPodTime int64) {
	for {
		for appId, appInfo := range n.Apps {
			_, err := os.Stat(appInfo.CgroupPath)
			if errors.Is(err, os.ErrNotExist) {
				err := n.deleteAppInfo(appId)
				if err != nil {
					klog.Warningf("delete appId failed %v: %s", err, appId)
				}
			}
		}
		time.Sleep(time.Duration(clearPodTime) * time.Second)
	}
}

func (n *NetEngine) deleteAppInfo(AppId string) error {
	if _, OK := n.Apps[AppId]; !OK {
		return errors.New("app not registered or already deleted")
	}
	for _, nic := range n.nics {
		// check whether this plugin are configured
		p, ok := n.plugins[nic.IpAddr]
		if !ok {
			return errors.New(nic.IpAddr + " haven't been configured")
		}
		err := p.UnRegisterApp(n.Apps[AppId])
		if err != nil {
			klog.Error("Unregister fails:", err)
		}
	}

	delete(n.podsVethCache, AppId)
	err := DeleteAppInfo(n, n.Apps[AppId].AppName)
	if err != nil {
		klog.Errorf("persistent deleteAppInfo %v failed, and the reason is: %v", AppId, err)
	} else {
		klog.V(utils.DBG).Info("persistent deleteAppInfo: %v", AppId)
	}
	delete(n.Apps, AppId)

	return nil
}

func ParseServiceInfoRequestString(s string) (ServiceInfo, error) {
	var info ServiceInfo
	if s == "" {
		return info, errors.New("empty ServiceInfo request")
	}

	err := json.Unmarshal([]byte(s), &info)
	if err == nil {
		klog.V(utils.DBG).Infof("parseServiceInfoRequestString success %v\n", info)
	} else {
		klog.Errorf("parseServiceInfoRequestString failed because string format is wrong, %s\n", s)
		return info, err
	}

	return info, nil
}

func ParseConfigureRequestString(s string) (ConfigureInfo, error) {
	var configure ConfigureInfo
	if s == "" {
		return configure, errors.New("empty configure request")
	}

	err := json.Unmarshal([]byte(s), &configure)
	if err == nil {
		klog.V(utils.DBG).Infof("parseConfigureRequest success %v\n", configure)
	} else {
		klog.Errorf("parseConfigureRequest failed because string format is wrong, %s\n", s)
		return configure, err
	}

	return configure, nil
}

func ParseRegisterAppRequestString(s string) (RegisterAppInfo, error) {
	var reg RegisterAppInfo
	if s == "" {
		return reg, errors.New("empty register request")
	}

	err := json.Unmarshal([]byte(s), &reg)
	if err == nil {
		klog.V(utils.DBG).Infof("parseRegisterAppRequest success %v\n", reg)
	} else {
		klog.Errorf("parseRegisterAppRequest failed because string format is wrong, %s\n", s)
		return reg, err
	}

	return reg, nil
}

func (n *NetEngine) rebuild() error {
	//todo: rebuild server data struct
	apps, err := GetAllAppInfos(n, "")
	if err != nil {
		klog.Errorf("getAllAppInfos %v from store error: %v", apps, err)
	} else {
		n.Apps = apps
		klog.V(utils.DBG).Info("persistent: rebuild apps: ", apps)
	}

	nics, err := GetNics(n)
	if err != nil {
		klog.Errorf("GetNics %v from store error: %v", nics, err)
	} else {
		n.nics = nics
		klog.V(utils.DBG).Info("persistent: rebuild nics: ", nics)
	}

	groups, err := GetGroups(n)
	if err != nil {
		klog.Errorf("getGroups %v from store error: %v", groups, err)
	} else {
		n.groups = groups
		klog.V(utils.DBG).Info("persistent: rebuild groups: ", groups)
	}

	groupLimit, err := GetGroupLimit(n)
	if err != nil {
		klog.Errorf("getGroupLimit %v from store error: %v", groupLimit, err)
	} else {
		n.groupLimit = groupLimit
		klog.V(utils.DBG).Info("persistent: rebuild groupLimit: ", groupLimit)
	}

	cnis, err := GetCNIs(n)
	if err != nil {
		klog.Errorf("getCNIs %v from store error: %v", cnis, err)
	} else {
		if len(cnis) > 0 {
			n.cnis = cnis
			klog.V(utils.DBG).Info("persistent: rebuild cnis: ", cnis)
		}
	}

	for i, nic := range n.nics {
		isAdq := nic.InterfaceType == ADQ
		if isAdq {
			p, _, err := NewAdqPlugin(nic.InterfaceName, nic.IpAddr, n.cnis[0], n.groups)
			if err != nil {
				p, _, err = NewGenericPlugin(nic.InterfaceName, n.cnis[0], n.groups)
				if err != nil {
					return err
				}
				n.nics[i].InterfaceType = NonAdq
			} else {
				n.nics[i].InterfaceType = ADQ
			}
			n.plugins[nic.IpAddr] = p
		} else {
			klog.V(utils.INF).Infof("nic %s is generic plugin", nic.InterfaceName)
			p, _, err := NewGenericPlugin(nic.InterfaceName, n.cnis[0], n.groups)
			if err != nil {
				return err
			}
			n.nics[i].InterfaceType = NonAdq
			n.plugins[nic.IpAddr] = p
		}
	}

	// real bw cache
	n.prevMonitor = make(map[string]SR)
	n.podsVethCache = make(map[string][]Veth)

	groupSettable, err := GetGroupSettable(n)
	if err != nil {
		klog.Errorf("getGroupSettable %v from store error: %v", groupSettable, err)
	} else {
		n.groupSettable = groupSettable
		klog.V(utils.DBG).Info("persistent: rebuild config: ", groupSettable)
	}
	return nil
}

func checkAdq(nic string, groups []Group) bool {
	// todo: detect adapter, call adqsetup or check config ?
	// supposing nic is interface name
	exist, err := HasIngress(nic)
	if err != nil {
		klog.Error("find exsiting ingress qdisc fails:", err)
		return false
	} else if exist {
		klog.V(utils.INF).Infof("nic %v has ingress qdisc, cannot setup adq environment", nic)
		return false
	}
	command := []string{"--verbose", "create", "[globals]", "dev", nic, "priority", "skbedit"}

	var gaGroupCommand []string
	var beGroupCommand []string
	for _, group := range groups {
		if group.Queues == 0 {
			klog.V(utils.INF).Info("Group reserved queue number equal to 0")
			return false
		}
		if group.GroupType == GROUP_SEPARATE_LAZY {
			gaGroupCommand = []string{"[GA]", "queues", strconv.FormatUint(group.Queues, 10),
				"pollers", strconv.FormatUint(Pollers, 10), "min_rate", strconv.FormatUint(group.Pool, 10)}
		} else if group.GroupType == GROUP_SHARE {
			beGroupCommand = []string{"[BE]", "queues", strconv.FormatUint(group.Queues, 10),
				"pollers", strconv.FormatUint(Pollers, 10)}
		}
	}
	if len(gaGroupCommand) == 0 || len(beGroupCommand) == 0 {
		klog.Error("group configuration error")
	}
	command = append(command, gaGroupCommand...)
	command = append(command, beGroupCommand...)
	klog.V(utils.DBG).Info(command)
	cmd := exec.Command("adqsetup", command...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		klog.Errorf("adqsetup execution failed: %v, error message: %s\n", err, stderr.String())
		return false
	}

	return true
}

func HasNic(nics []string, nic string) bool {
	for _, n := range nics {
		if n == nic {
			return true
		}
	}
	return false
}

// UpdateConfig in server, if changed or new, reset adq or non-adq setup
func (n *NetEngine) updateConfig(config ConfigureInfo) (string, error) {
	var groupSettable string
	n.groups = config.Groups
	// now only use main cni
	n.cnis[0] = CNI{Name: NetworkConfig.CNI, Gateway: NetworkConfig.Backend}
	n.nics = config.NodeIps
	if n.cnis[0].Gateway == "" {
		n.cnis[0].Gateway = n.nics[0].InterfaceName
	}
	err := AddNic(n, n.nics[0])
	if err != nil {
		klog.Errorf("persistent addNic %v failed, and the reason is: %v", n.nics[0], err)
	} else {
		klog.V(utils.DBG).Info("persistent addNic: %v", n.nics[0])
	}
	for _, v := range n.groups {
		err := AddGroup(n, v)
		if err != nil {
			klog.Errorf("persistent addGroup %v failed, and the reason is: %v", v, err)
		} else {
			klog.V(utils.DBG).Info("persistent addGroup: %v", v)
		}
	}
	err = AddCni(n, n.cnis[0])
	if err != nil {
		klog.Errorf("persistent addCni %v failed, and the reason is: %v", n.cnis[0], err)
	} else {
		klog.V(utils.DBG).Info("persistent addCni: %v", n.cnis[0])
	}

	// todo: get all new or updated nics and reset adq or nor-adq setup
	for i, nic := range n.nics {
		isAdq := checkAdq(nic.InterfaceName, config.Groups)
		if isAdq {
			klog.V(utils.INF).Info("Is adq plugin")
			p, gs, err := NewAdqPlugin(nic.InterfaceName, nic.IpAddr, n.cnis[0], n.groups)
			groupSettable = gs
			if err != nil {
				p, gs, err = NewGenericPlugin(nic.InterfaceName, n.cnis[0], n.groups)
				groupSettable = gs
				if err != nil {
					n.groupSettable = groupSettable
					return groupSettable, err
				}
				n.nics[i].InterfaceType = NonAdq
			} else {
				n.nics[i].InterfaceType = ADQ
			}
			n.plugins[nic.IpAddr] = p
		} else {
			klog.V(utils.INF).Info("Is genernic plugin")
			p, gs, err := NewGenericPlugin(nic.InterfaceName, n.cnis[0], n.groups)
			groupSettable = gs
			if err != nil {
				n.groupSettable = groupSettable
				return groupSettable, err
			}
			n.nics[i].InterfaceType = NonAdq
			n.plugins[nic.IpAddr] = p
		}
	}
	// todo: get all new or updated groups, re-binding app to new groups
	for _, nic := range n.nics {
		err := n.plugins[nic.IpAddr].Configure(config.Groups)
		if err != nil {
			n.groupSettable = groupSettable
			return groupSettable, err
		}
	}
	n.groupSettable = groupSettable
	err = AddGroupSettable(n, groupSettable)
	if err != nil {
		klog.Errorf("store groupSettable %v failed: %v", groupSettable, err)
	} else {
		klog.V(utils.DBG).Info("persistent addGroupSettable: %v", groupSettable)
	}
	return groupSettable, nil
}

func getInterfaceByIP(ip net.IP) (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err == nil {
			for _, addr := range addrs {
				iip, _, err := net.ParseCIDR(addr.String())
				if err == nil {
					if iip.Equal(ip) {
						return iface.Name, nil
					}
				} else {
					continue
				}
			}
		} else {
			continue
		}
	}
	return "", errors.New("couldn't find a interface for the ip")
}

func getNicMaxQueue(iface string) (uint64, error) {
	ethHandle, err := ethtool.NewEthtool()
	if err != nil {
		return 0, err
	}
	defer ethHandle.Close()
	channels, err := ethHandle.GetChannels(iface)
	klog.V(utils.DBG).Info(channels)
	if err != nil {
		return 0, err
	}
	return uint64(channels.MaxCombined), nil
}

func (s *NetEngine) GetServiceInfo(ctx context.Context, req *pb.ServiceInfoRequest) (*pb.ServiceInfoResponse, error) {
	klog.V(utils.INF).Info("Envoke GetServiceInfo:", req.Req, "  Type:", req.IoiType)
	if req.IoiType != utils.NetIOIType {
		return &pb.ServiceInfoResponse{Status: pb.ResponseState_FAIL}, fmt.Errorf("get service info not for network io service")
	}

	var cni string
	var err error
	// Detect CNI
	if !s.option.IsBridgeCNI {
		cni, err = DetectCNI()
		if err != nil || cni == "" {
			klog.Errorf("DetectCNI error, %v", err)
			cni = Bridge
			BridgeMode = true
		}
	} else {
		cni = Bridge
	}

	var backend string
	if cni == Flannel {
		backend, err = DetectBackendFlannel()
		if err != nil {
			klog.Errorf("DetectBackendFlannel error, %v", err)
			return &pb.ServiceInfoResponse{Status: pb.ResponseState_FAIL}, err
		}
	} else if cni == Calico {
		backend, err = DetectBackendCalico()
		if err != nil {
			klog.Errorf("DetectBackendCalico error, %v", err)
			return &pb.ServiceInfoResponse{Status: pb.ResponseState_FAIL}, err
		}
	} else if cni == Cilium {
		backend, err = DetectBackendCilium()
		if err != nil {
			klog.Errorf("DetectBackendCilium error, %v", err)
			return &pb.ServiceInfoResponse{Status: pb.ResponseState_FAIL}, err
		}
	} else if cni == Bridge {
		BridgeMode = true
	} else {
		klog.Errorf("cni is not flannel or calico or cilium, cni: %s", cni)
		BridgeMode = true
		cni = Bridge
	}

	ipaddr, err := ParseServiceInfoRequestString(req.Req)
	if err != nil {
		return &pb.ServiceInfoResponse{Status: pb.ResponseState_FAIL}, fmt.Errorf("parse req error:%v", req)
	}
	ip := net.ParseIP(ipaddr.IP)
	if ip == nil {
		return &pb.ServiceInfoResponse{Status: pb.ResponseState_FAIL}, fmt.Errorf("parse ip error:%v", ipaddr)
	}
	iface, err := getInterfaceByIP(ip)
	if err != nil {
		return &pb.ServiceInfoResponse{Status: pb.ResponseState_FAIL}, fmt.Errorf("get interface by IP error:%v", err)
	}
	klog.V(utils.DBG).Info("Iface Name:", iface)
	macPath := fmt.Sprintf(MacPath, iface)
	var mac string
	macaddr, err := os.ReadFile(macPath)
	if err != nil {
		klog.Errorf("cannot get macaddr:%v", err)
		mac = DefaultMacAddr
	} else {
		mac = strings.TrimSpace(string(macaddr))
	}
	speedPath := fmt.Sprintf(SpeedPath, iface)
	speedBytes, err := os.ReadFile(speedPath)
	var speed string
	if err != nil {
		klog.Errorf("Can't read nic speed from %s", speedPath)
		speed = DefaultSpeed
	} else {
		speed = strings.TrimSpace(string(speedBytes))
		speedInt, err := strconv.ParseInt(speed, 10, 64)
		if err != nil || speedInt < 0 {
			speed = DefaultSpeed
		}
	}

	speedConv, err := strconv.ParseUint(speed, 10, 64)
	if err != nil {
		klog.Errorf("cannot parse speed to uint64:%v", err)
	}
	maxQueuePairs, err := getNicMaxQueue(iface)
	if err != nil {
		klog.Warningf("cannot get maximum queue pairs:%v", err)
		maxQueuePairs = 0
	}

	NetworkConfig.InterfaceName = iface
	NetworkConfig.MacAddr = mac
	if s.option.Speed != 0 {
		speedConv = s.option.Speed
	}
	NetworkConfig.DefaultSpeed = speedConv
	NetworkConfig.QueueNum = maxQueuePairs
	NetworkConfig.CNI = cni
	NetworkConfig.Backend = backend

	nicInfo := utils.NicInfo{
		Name:     iface,
		Macaddr:  mac,
		TotalBPS: float64(speedConv),
		QueueNum: int(maxQueuePairs),
	}
	data, err := json.Marshal(nicInfo)
	if err != nil {
		return &pb.ServiceInfoResponse{Status: pb.ResponseState_FAIL}, fmt.Errorf("marshal config info fails: %v", err)
	}
	klog.V(utils.DBG).Info("data:", string(data))
	resp := &pb.ServiceInfoResponse{
		Type:          req.IoiType,
		Configuration: string(data),
	}
	return resp, nil
}

func (n *NetEngine) Configure(ctx context.Context, req *pb.ConfigureRequest) (*pb.ConfigureResponse, error) {
	klog.V(utils.INF).Info("Envoke configure:", req.Configure)
	if req.Configure == "" {
		return &pb.ConfigureResponse{Status: pb.ResponseState_FAIL}, nil
	}

	if req.Type == utils.INTERVAL {
		var interval utils.IntervalConfigInfo
		err := json.Unmarshal([]byte(req.Configure), &interval)
		if err == nil {
			klog.Infof("parseConfigureRequest Interval success %v\n", interval)
		} else {
			klog.Errorf("parseConfigureRequest Interval failed because string format is wrong, %s\n", n)
			return &pb.ConfigureResponse{Status: pb.ResponseState_FAIL}, nil
		}
		n.Interval = interval
		err = SaveInterval(n, interval)
		if err != nil {
			klog.Errorf("store config log level err: %v", err)
		}
		klog.Infof("Configure Interval %s.", n.Interval)
		return &pb.ConfigureResponse{Status: pb.ResponseState_OK}, nil
	} else {
		config, err := ParseConfigureRequestString(req.Configure)
		if err != nil {
			return &pb.ConfigureResponse{Status: pb.ResponseState_FAIL}, nil
		}

		if n.groupSettable != "" {
			klog.V(utils.INF).Info("node-agent re-send configure request:", n.groupSettable)
			return &pb.ConfigureResponse{Status: pb.ResponseState_OK, ResponseInfo: n.groupSettable}, nil
		}
		info, err := n.updateConfig(config)
		if err != nil {
			klog.Errorf("handle configure request err: %v", err)
		}
		klog.V(utils.DBG).Info("Configure resp:", info)
		return &pb.ConfigureResponse{Status: pb.ResponseState_OK, ResponseInfo: info}, nil
	}
}

func (n *NetEngine) SetAppLimit(ctx context.Context, req *pb.SetAppLimitRequest) (*pb.SetAppLimitResponse, error) {
	klog.V(utils.INF).Info("Evoke SetAppLimit:", req.AppId, "  Limit:", req.Limit)
	if req.AppId == "" {
		return &pb.SetAppLimitResponse{Status: pb.ResponseState_FAIL}, errors.New("req appid is null")
	}
	var group, app string
	groupApp := strings.Split(req.AppId, ".")
	// Todo: Also handle len=1 to set limit in ifb group level
	if len(groupApp) == 1 {
		group = groupApp[0]
	} else if len(groupApp) == 2 {
		group = groupApp[0]
		app = groupApp[1]
	} else {
		klog.Error("AppId format error")
		return &pb.SetAppLimitResponse{Status: pb.ResponseState_FAIL}, errors.New("appId format error")
	}
	_, appOk := n.Apps[app]
	grpOk := n.findGroup(group)
	if app != "" && !appOk {
		return &pb.SetAppLimitResponse{Status: pb.ResponseState_FAIL}, fmt.Errorf("app %s is unregistered", app)
	}
	if appOk || grpOk {
		//update appinfo in server
		for _, limit := range req.Limit {
			if limit.In == "" && limit.Out == "" {
				klog.Error("Invalid limit values: in and out are both absent")
				return &pb.SetAppLimitResponse{Status: pb.ResponseState_FAIL}, errors.New("invalid limit values: in and out are both absent")
			}
			bwlimit := BWLimit{
				Id:      limit.Id,
				Ingress: limit.In,
				Egress:  limit.Out,
			}
			if appOk {
				// s.apps[app].Limits = append(s.apps[app].Limits, bwlimit)
				n.Apps[app].Limits = []BWLimit{bwlimit}
			} else if limit.In != "" && limit.Out != "" {
				n.groupLimit[group] = bwlimit
				err := AddGroupLimit(n, group, bwlimit)
				if err != nil {
					klog.Errorf("persistent addGroupLimit failed, group: %v limit: %v, and the reason is: %v", group, bwlimit, err)
				} else {
					klog.V(utils.DBG).Info("persistent addGroupLimit, group: %v, limit: %v", group, bwlimit)
				}
			}
		}

		for _, nic := range n.nics {
			for _, limit := range req.Limit {
				bwlimit := BWLimit{
					Id:      limit.Id,
					Ingress: limit.In,
					Egress:  limit.Out,
				}
				if appOk {
					err := n.plugins[nic.IpAddr].SetLimit(group, n.Apps[app], bwlimit)
					if err != nil {
						return &pb.SetAppLimitResponse{Status: pb.ResponseState_FAIL}, err
					}
				} else {
					err := n.plugins[nic.IpAddr].SetLimit(group, nil, bwlimit)
					if err != nil {
						return &pb.SetAppLimitResponse{Status: pb.ResponseState_FAIL}, err
					}
				}
			}
		}

		return &pb.SetAppLimitResponse{Status: pb.ResponseState_OK}, nil
	} else {
		return &pb.SetAppLimitResponse{Status: pb.ResponseState_FAIL}, errors.New("app is not registered")
	}
}

func (app *AppInfo) Initialize() {
	app.Devices = app.GetDevices(false)
}
func (app *AppInfo) GetDevices(retry bool) map[string]string {
	var i int
	var retryCount int = 3
	var EthName string
	mutex.Lock()
	defer mutex.Unlock()
	if len(app.Devices) > 0 {
		return app.Devices
	}
	for i = 0; i < retryCount; i++ {
		eth0, err := GetDefaultGatewayInterfaceInNetNS(app.NetNS)
		if err != nil {
			klog.Errorf("get master nic error is %v", err)
			if retry && i == retryCount-1 {
				EthName = app.IfbName
				err = nil
			}
		} else {
			EthName = eth0.Name
		}
		if err == nil {
			v, err1 := GetInterfaceIpv4AddrInNamespace(eth0.Name, app.NetNS)
			if err1 == nil {
				klog.Infof("GetDevices GetInterfaceIpv4AddrInNamespace Ethname:%v", EthName)
				app.Devices[EthName] = v
				return app.Devices
			} else {
				klog.Errorf("GetInterfaceIpv4AddrInNamespace error %v", err)
			}
		}
		if !retry {
			break
		}
		if i < retryCount-1 {
			time.Sleep(5 * time.Second)
		}
	}
	if i == retryCount {
		klog.Infof("GetDevices Retry Ethname:%v", EthName)
		app.Devices[EthName] = "0.0.0.0"
	}
	return app.Devices
}

func (n *NetEngine) RegisterApp(ctx context.Context, req *pb.RegisterAppRequest) (*pb.RegisterAppResponse, error) {
	klog.V(utils.INF).Info("Evoke RegisterApp:", req.AppName, " ", req.AppInfo)
	// check req is valid
	if req.AppName == "" || req.AppInfo == "" {
		return &pb.RegisterAppResponse{
			AppId:  req.AppName,
			Status: pb.ResponseState_FAIL,
		}, errors.New("app name or app info should not be empty")
	}

	// check whether this app is registered
	if _, OK := n.Apps[req.AppName]; OK {
		return &pb.RegisterAppResponse{
			AppId:  req.AppName,
			Status: pb.ResponseState_FAIL,
		}, errors.New("app already registered")
	}

	app, err := ParseRegisterAppRequestString(req.AppInfo)
	if err != nil {
		return &pb.RegisterAppResponse{
			AppId:  req.AppName,
			Status: pb.ResponseState_FAIL,
		}, err
	}
	if app.Group == "" || app.NetNS == "" || app.CgroupPath == "" || app.Devices == nil || len(app.Devices) == 0 {
		return &pb.RegisterAppResponse{
			AppId:  req.AppName,
			Status: pb.ResponseState_FAIL,
		}, errors.New("app group or app netns or App Devices should not be empty")
	}

	//eth0, err := GetDefaultGatewayInterfaceInNetNS(app.NetNS)
	//if err != nil {
	//	klog.Errorf("get master nic error is %v", err)
	//	return &pb.RegisterAppResponse{
	//		AppId:  req.AppName,
	//		Status: pb.ResponseState_FAIL,
	//	}, errors.New("can't find default gateway")
	//}
	devices := make(map[string]string)
	v, err := GetInterfaceIpv4AddrInNamespace(Eth0, app.NetNS)
	if err != nil {
		klog.Errorf("can not get pod's IP err is %v", err)
		devices[Eth0] = "0.0.0.0"
	} else {
		devices[Eth0] = v
	}

	a := AppInfo{
		AppName:    req.AppName,
		Group:      app.Group,
		NetNS:      app.NetNS,
		CgroupPath: app.CgroupPath,
		Devices:    make(map[string]string),
		IfbName:    n.option.Ifname,
	}
	a.Initialize()
	klog.V(utils.DBG).Infof("handle app %s register events, %v", req.AppName, a.Devices)
	n.Apps[req.AppName] = &a

	for _, nic := range n.nics {
		// check whether this plugin are configured
		p, ok := n.plugins[nic.IpAddr]
		if !ok {
			return &pb.RegisterAppResponse{
				AppId:  req.AppName,
				Status: pb.ResponseState_FAIL,
			}, fmt.Errorf("%s is not been configured", nic)
		}
		err := p.RegisterApp(n.Apps[req.AppName])
		if err != nil {
			return &pb.RegisterAppResponse{
				AppId:  req.AppName,
				Status: pb.ResponseState_FAIL,
			}, err
		}
	}

	err = AddAppInfo(n, a)
	if err != nil {
		klog.Errorf("store app %v failed: %v", a, err)
	} else {
		klog.V(utils.DBG).Info("persistent addAppInfo: %v", a)
	}

	return &pb.RegisterAppResponse{
		AppId:  req.AppName,
		Status: pb.ResponseState_OK,
	}, nil

}

func (n *NetEngine) UnRegisterApp(ctx context.Context, req *pb.UnRegisterAppRequest) (*pb.UnRegisterAppResponse, error) {
	klog.V(utils.INF).Info("Evoke UnRegisterApp:", req.AppId)
	if req.AppId == "" {
		return &pb.UnRegisterAppResponse{Status: pb.ResponseState_FAIL}, errors.New("req appid is null")
	}

	err := n.deleteAppInfo(req.AppId)
	if err != nil {
		return &pb.UnRegisterAppResponse{Status: pb.ResponseState_FAIL}, err
	}

	return &pb.UnRegisterAppResponse{Status: pb.ResponseState_OK}, nil
}

func createMsg(ioiType int32, data map[string][]Veth) *pb.AppsBandwidth {
	BWs := make(map[string]*pb.AppNetworkBandwidthInfo)

	for podid, veths := range data {
		var bwinfo []*pb.NetworkBandwidthInfo
		for _, veth := range veths {
			ingress := strconv.FormatFloat(veth.Traffic.Receive, 'f', -1, 64)
			egress := strconv.FormatFloat(veth.Traffic.Send, 'f', -1, 64) // for host veth pair
			nbi := &pb.NetworkBandwidthInfo{
				Id:        veth.Veth,
				IsIngress: true,
				Value:     egress,
			}
			bwinfo = append(bwinfo, nbi)
			nbe := &pb.NetworkBandwidthInfo{
				Id:        veth.Veth,
				IsIngress: false,
				Value:     ingress,
			}
			bwinfo = append(bwinfo, nbe)
		}
		BWs[podid] = &pb.AppNetworkBandwidthInfo{BwInfo: bwinfo}
	}
	if ioiType == 2 {
		msg := pb.AppsBandwidth{
			IoiType: ioiType,
			Payload: &pb.AppsBandwidth_NetworkIoBw{
				NetworkIoBw: &pb.AppNetworkIOBandwidth{
					AppBwInfo: BWs,
				}},
		}
		return &msg
	}
	return nil
}

func (s *NetEngine) Subscribe(req *pb.SubscribeContext, stream pb.Ioiservice_SubscribeServer) error {
	// s.Stream = stream
	for {
		// 1.get current bandwidth data
		num := len(s.Apps)
		if num == 0 {
			time.Sleep(time.Second * time.Duration(s.Interval.Interval))
		} else {
			if s.option.EnableSubscribe {
				currMonitor, err := s.getSndRcv()
				if err != nil {
					return err
				}
				// 2.send bandwidth data to client
				curData, err := s.getCurBandwidth(s.prevMonitor, currMonitor, s.Apps)
				if err != nil {
					return err
				}
				s.prevMonitor = currMonitor
				err = send(stream, req.IoiType, curData)
				if err != nil {
					return err
				}
			}
			time.Sleep(time.Second * time.Duration(s.Interval.Interval))
		}
	}
}

// get current net traffic
func (n *NetEngine) getSndRcv() (map[string]SR, error) {
	res := make(map[string]SR)
	proc := "/proc/net/dev"
	file, err := os.Open(proc)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	rd := bufio.NewReader(file)
	for {
		row, _, err := rd.ReadLine()
		if err == io.EOF {
			break
		}
		rowStr := string(row)
		line := strings.Fields(strings.TrimSpace(rowStr))
		if !strings.Contains(line[0], ":") {
			continue
		}
		nic := line[0][:len(line[0])-1]
		rcvBytes, err := strconv.Atoi(line[1])
		if err != nil {
			klog.Warning(err)
		}
		sndBytes, err := strconv.Atoi(line[9])
		if err != nil {
			klog.Warning(err)
		}
		res[nic] = SR{
			Send:    float64(sndBytes),
			Receive: float64(rcvBytes),
		}
	}
	return res, nil
}

func (s *NetEngine) getCurBandwidth(prev map[string]SR, curr map[string]SR, apps map[string]*AppInfo) (map[string][]Veth, error) {
	res := make(map[string][]Veth)
	for appid, app := range apps {
		// get veths of a pod
		var veths []Veth
		if vs, ok := s.podsVethCache[appid]; ok {
			veths = vs
		} else {
			for podNic := range app.Devices {
				hd, err := getHostInterface(podNic, app.NetNS)
				if err != nil {
					return nil, err
				}
				veths = append(veths, Veth{HostVeth: hd.Attrs().Name, Veth: podNic})
			}
			s.podsVethCache[appid] = veths
		}
		// calculate the traffic currently
		for _, v := range veths {
			prevData, ok1 := prev[v.HostVeth]
			currData, ok2 := curr[v.HostVeth]
			if !ok1 && !ok2 {
				klog.V(utils.DBG).Infof("the veth %v of pod %v is not exist", v.HostVeth, app.AppName)
			}
			if !ok1 && ok2 {
				klog.V(utils.DBG).Infof("the veth %v of pod %v stored initially", v.HostVeth, app.AppName)
			}
			if ok1 && !ok2 {
				klog.V(utils.DBG).Info("the veth %v of pod %v is deleted", v.HostVeth, app.AppName)
			}
			if ok1 && ok2 {
				ingress := math.Max(currData.Receive-prevData.Receive, 0)
				egress := math.Max(currData.Send-prevData.Send, 0)
				res[appid] = append(res[appid], Veth{HostVeth: v.HostVeth, Veth: v.Veth, Traffic: SR{Send: egress, Receive: ingress}})
			}
		}
	}
	return res, nil
}

func send(ss pb.Ioiservice_SubscribeServer, ioiType int32, data map[string][]Veth) error {
	instance := createMsg(ioiType, data)
	err := ss.Send(instance)
	if err != nil {
		klog.Warning("send failed", err)
	}
	return err
}

func (n *NetEngine) findGroup(group string) bool {
	for _, grp := range n.groups {
		if group == grp.Id {
			return true
		}
	}
	return false
}

func init() {
	a := service.GetService()
	if (a.EnginesSwitch & service.NetSwitch) == 0 {
		return
	}

	engine := &NetEngine{
		service.ServiceEngine{},
		nil,
		ServerInfo{},
		Option{},
		"",
		make(map[string]*AppInfo),
		[]Group{},
		make(map[string]BWLimit),   // group_id -> BWLimit
		[]NICInfo{},                // all the host network adapter ip
		[]CNI{},                    // multiple cni in the future, now use cnis[0] as main cni
		utils.IntervalConfigInfo{}, // current configuration interval
		make(map[string]Plugin),    // host network_ip => plugins
		make(map[string]SR),
		make(map[string][]Veth),
	}

	a.RegisterEngine(engine)
}
