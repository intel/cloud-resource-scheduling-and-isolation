/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package net

import (
	"encoding/json"
	"strconv"

	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
)

var (
	INGRESS_IFB_PREFIX string = "ingress_ifb"
	EGRESS_IFB_PREFIX  string = "egress_ifb"
	INGRESS_IP_OFF     int32  = 16
	EGRESS_IP_OFF      int32  = 12
)

// only for BE group now
type GenericPlugin struct {
	Plugin  CommonPlugin
	HostNic string
	cni     CNI
	groups  []Group
}

func (g *GenericPlugin) ConfigureEgressGroup(group Group) error {
	/*
		create and launch IFB
		add limit on IFB
	*/
	mtu, err := getMTU(g.HostNic)
	if err != nil {
		klog.Errorf("failed to get MTU. Reason: %v", err)
	}
	ifbDeviceName := EGRESS_IFB_PREFIX + "_" + group.Id
	if err = createIfb(ifbDeviceName, mtu); err != nil {
		klog.Errorf("failed to create ifb. Reason: %v", err)
		return err
	}
	if err = addIfbLimit(mbitToBit(group.Pool), ifbDeviceName); err != nil {
		klog.Errorf("failed to add limit on ifb. Reason: %v", err)
		return err
	}
	return nil
}

func (g *GenericPlugin) ConfigureIngressGroup(group Group) error {
	/*
		create and launch IFB
		add limit on IFB
		add qdisc ingress on specific nic
	*/
	var hostDevice netlink.Link
	// create ifb
	mtu, err := getMTU(g.HostNic)
	if err != nil {
		klog.Errorf("failed to get MTU. Reason: %v", err)
	}
	ifbDeviceName := INGRESS_IFB_PREFIX + "_" + group.Id
	if err = createIfb(ifbDeviceName, mtu); err != nil {
		klog.Errorf("failed to create ifb. Reason: %v", err)
		return err
	}
	if err = addIfbLimit(mbitToBit(group.Pool), ifbDeviceName); err != nil {
		klog.Errorf("failed to add limit on ifb. Reason: %v", err)
		return err
	}
	// add qdisc ingress
	if hostDevice, err = netlink.LinkByName(g.cni.Gateway); err != nil {
		klog.Errorf("failed to get hostDevice. Reason: %v", err)
		return err
	}
	if err = operateIngressQdisc(hostDevice, OP_ADD); err != nil {
		klog.Errorf("failed in operateIngressQdisc. Reason: %v", err)
		return err
	}
	return nil
}

func IsGroupSettable(group int) bool {
	if group == GROUP_SHARE && !BridgeMode {
		return true
	}

	return false
}

func (g *GenericPlugin) Configure(groups []Group) error {
	klog.V(utils.INF).Info("generic plugin config start")
	klog.V(utils.DBG).Infof("HostNic is %s, cni is %v", g.HostNic, g.cni)
	for _, group := range groups {
		klog.V(utils.DBG).Info(group)
		// Only configure egress/ingress ifb in Bridge mode and GroupType == Group_SHARE
		if !IsGroupSettable(group.GroupType) {
			continue
		}
		if group.GroupType == GROUP_SEPARATE_LAZY || group.GroupType == GROUP_SEPARATE_DYNAMIC {
			continue
		}

		klog.V(utils.DBG).Info("Group type:", group.GroupType, "  id:", group.Id)
		// BW get from node agent or detect by service
		err := g.ConfigureEgressGroup(group)
		if err != nil {
			klog.Errorf("group %s config egress group err %v", group.GroupType, err)
		}
		err = g.ConfigureIngressGroup(group)
		if err != nil {
			klog.Errorf("group %s config ingress group err %v", group.GroupType, err)
		}
	}
	return nil
}

func (g *GenericPlugin) RegisterApp(app *AppInfo) error {
	/*
		add qdisc ingress
		add filter to redirect traffic
	*/
	klog.V(utils.INF).Info("generic plugin RegisterApp")
	klog.V(utils.DBG).Infof("app info %v", app)
	if appType := g.getGroupType(app.Group); appType == -1 || appType == GROUP_SEPARATE_LAZY || appType == GROUP_SEPARATE_DYNAMIC || !IsGroupSettable(appType) {
		klog.Infof("Generic Plugin doesn't support GA group bandwidth change and BE app bandwidth change")
		return nil
	}
	if err := operateApp(app, OP_ADD, g.cni.Gateway); err != nil {
		return err
	}
	return nil
}

func (g *GenericPlugin) UnRegisterApp(app *AppInfo) error {
	/*
		delete qdisc ingress
		delete filter to redirect traffic
	*/
	klog.V(utils.INF).Info("generic plugin UnRegisterApp")
	if appType := g.getGroupType(app.Group); appType == -1 || appType == GROUP_SEPARATE_LAZY || appType == GROUP_SEPARATE_DYNAMIC || !IsGroupSettable(appType) {
		if err := operateGALimit(app, 0, 0, OP_DEL); err != nil {
			return err
		}
		return nil
	}
	if err := operateApp(app, OP_DEL, g.cni.Gateway); err != nil {
		return err
	}
	return nil
}

func (g *GenericPlugin) SetLimit(group string, app *AppInfo, limit BWLimit) error {
	/*
		BE: Grp (handle) / App; add limit on IFB
		GA: Grp / App (handle); add qdisc on host interface / in container network
		BT: Grp (handle) / App (handle);
	*/
	var limitIn float64
	var limitOut float64
	var err error
	klog.V(utils.INF).Info("generic plugin SetLimit")
	appType := g.getGroupType(group)
	if appType == -1 || appType == GROUP_SEPARATE_LAZY && app == nil || appType == GROUP_SHARE && app != nil && IsGroupSettable(appType) {
		klog.V(utils.INF).Info("Generic Plugin doesn't support GA group bandwidth change and BE app bandwidth change")
		return nil
	}
	if limit.Ingress != "" {
		limitIn, err = strconv.ParseFloat(limit.Ingress, 64)
		if err != nil {
			klog.Error("Ingress bandwidth limit value fails to convert to uint64:", err)
			return err
		}
		if limitIn == 0 {
			limitIn = 1
		}
	}
	if limit.Egress != "" {
		limitOut, err = strconv.ParseFloat(limit.Egress, 64)
		if err != nil {
			klog.Error("Egress bandwidth limit value fails to convert to uint64:", err)
			return err
		}
		if limitOut == 0 {
			limitOut = 1
		}
	}
	// BE: Throttle in group level
	if (appType == GROUP_SHARE && app == nil && IsGroupSettable(appType)) || (appType == GROUP_SEPARATE_DYNAMIC && app == nil && IsGroupSettable(appType)) {
		if limitIn > 0 {
			if limitIn == utils.MAX_BT_LIMIT {
				ifbDevice, err := netlink.LinkByName(INGRESS_IFB_PREFIX + "_" + group)
				if err != nil {
					return err
				}
				err = delQdiscIfExist(ifbDevice.Attrs().Index, "tbf")
				if err != nil {
					return err
				}
			} else {
				if err := addIfbLimit(mbitToBit(uint64(limitIn)), INGRESS_IFB_PREFIX+"_"+group); err != nil {
					klog.Errorf("failed to add limit on ifb. Reason: %v", err)
					return err
				}
			}
		}
		if limitOut > 0 {
			if limitOut == utils.MAX_BT_LIMIT {
				ifbDevice, err := netlink.LinkByName(EGRESS_IFB_PREFIX + "_" + group)
				if err != nil {
					return err
				}
				err = delQdiscIfExist(ifbDevice.Attrs().Index, "tbf")
				if err != nil {
					return err
				}
			} else {
				if err := addIfbLimit(mbitToBit(uint64(limitOut)), EGRESS_IFB_PREFIX+"_"+group); err != nil {
					klog.Errorf("failed to add limit on ifb. Reason: %v", err)
					return err
				}
			}
		}
	} else if (appType == GROUP_SEPARATE_LAZY && app != nil && !IsGroupSettable(appType)) || (appType == GROUP_SEPARATE_DYNAMIC && app != nil && !IsGroupSettable(appType)) || !IsGroupSettable(appType) {
		// GA: Throttle in app level
		if err := operateGALimit(app, limitIn, limitOut, OP_ADD); err != nil {
			return err
		}
	}

	return nil
}

func NewGenericPlugin(nic string, c CNI, group []Group) (Plugin, string, error) {
	//todo: init ifb and qdisc
	groupSettable := ConfigureRespInfo{
		Groups: []GroupSettable{},
	}
	for _, grp := range group {
		if grp.GroupType == GROUP_SEPARATE_LAZY {
			groupSettable.Groups = append(groupSettable.Groups, GroupSettable{Id: grp.Id, Settable: false})
		}
		if grp.GroupType == GROUP_SEPARATE_DYNAMIC {
			groupSettable.Groups = append(groupSettable.Groups, GroupSettable{Id: grp.Id, Settable: false})
		}
		if grp.GroupType == GROUP_SHARE {
			if BridgeMode {
				groupSettable.Groups = append(groupSettable.Groups, GroupSettable{Id: grp.Id, Settable: false})
			} else {
				groupSettable.Groups = append(groupSettable.Groups, GroupSettable{Id: grp.Id, Settable: true})
			}
		}
	}
	grpSettableStr, err := json.Marshal(groupSettable)
	if err != nil {
		klog.Error("failed to transfer groupSettable to string:", err)
	}
	return &GenericPlugin{Plugin: NewCommonPlugin(), HostNic: nic, cni: c, groups: group}, string(grpSettableStr), nil
}

func (g *GenericPlugin) getGroupType(group string) int {
	for _, grp := range g.groups {
		if group == grp.Id {
			return grp.GroupType
		}
	}
	return -1
}
