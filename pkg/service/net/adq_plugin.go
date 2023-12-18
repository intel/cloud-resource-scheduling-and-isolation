/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package net

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
)

type AdqPlugin struct {
	plugin CommonPlugin
	cni    CNI
	nic    string
	groups []Group
}

func (a *AdqPlugin) Configure(groups []Group) error {
	klog.V(utils.DBG).Info("Add clsact qdisc")
	// hostDevice, err := netlink.LinkByName(a.nic)
	// if err != nil {
	// 	return err
	// }
	// err = operateClsactQdisc(hostDevice, OP_ADD)
	// return err
	return nil
}

func (a *AdqPlugin) RegisterApp(app *AppInfo) error {
	klog.V(utils.INF).Info("adq plugin RegisterApp")
	// set ingress and egress TC(Egress limited in group)
	err := a.AddEgressFilter(app)
	if err != nil {
		return err
	}
	err = a.AddIngressFilter(app)
	if err != nil {
		return err
	}

	return nil
}

func (a *AdqPlugin) UnRegisterApp(app *AppInfo) error {
	klog.V(utils.INF).Info("adq plugin UnRegisterApp")
	var podNic string
	for k, ip := range app.Devices {
		err := a.DelADQFilter(net.ParseIP(ip))
		if err != nil {
			return err
		}
		podNic = k
	}
	// delete rate limit tbf qdisc
	if a.getGroupType(app.Group) == GROUP_SEPARATE_LAZY || a.getGroupType(app.Group) == GROUP_SEPARATE_DYNAMIC || a.getGroupType(app.Group) == GROUP_SHARE {
		// for ingress, delete qdisc for host interface
		if err := TCDeleteTbfHost(podNic, app.NetNS); err != nil {
			klog.Errorf("failed to delete tbf. Reason: %v", err)
			return err
		}
		// for egress, add qdisc in container network
		err := TCDeleteTbfContainer(podNic, app.NetNS)
		if err != nil {
			klog.Errorf("failed in operateQiscInCnet. Reason: %v", err)
			return err
		}
	}
	return nil
}

func (a *AdqPlugin) SetLimit(group string, app *AppInfo, limit BWLimit) error {
	klog.V(utils.INF).Info("adq plugin SetLimit")
	var podNic string
	if app == nil {
		klog.V(utils.DBG).Info("adq network io server plugin skips group level SetLimit handling")
		return nil
	}
	for k := range app.Devices { // a nic for a pod
		podNic = k
		break
	}
	veth, err := getHostInterface(podNic, app.NetNS)
	if err != nil {
		klog.Errorf("failed in getHostInterface. Reason: %v", err)
		return err
	}
	if a.getGroupType(app.Group) == GROUP_SEPARATE_LAZY || a.getGroupType(app.Group) == GROUP_SEPARATE_DYNAMIC || a.getGroupType(app.Group) == GROUP_SHARE {
		klog.V(utils.DBG).Info("ADQ plugin Set limit")
		// for ingress, add qdisc for host interface
		if limit.Ingress != "" {
			limitIn, err := strconv.ParseFloat(limit.Ingress, 64)
			if err != nil {
				klog.Error("Ingress bandwidth limit value fails to convert to uint64:", err)
				return err
			}
			if limitIn == 0 {
				limitIn = 1
			}
			if limitIn == utils.MAX_BT_LIMIT {
				err := delQdiscIfExist(veth.Attrs().Index, "tbf")
				if err != nil {
					klog.Errorf("delete app %v ingress limit error: %v", app.AppName, err)
					return err
				}
			} else if limitIn > 0 {
				if err = createTBF(mbitToBit(uint64(limitIn)), mbitToBit(8), veth.Attrs().Index); err != nil {
					klog.Errorf("failed to create tbf. Reason: %v", err)
					return err
				}
			}
		}
		// for egress, add qdisc in container network
		if limit.Egress != "" {
			limitOut, err := strconv.ParseFloat(limit.Egress, 64)
			if err != nil {
				klog.Error("Egress bandwidth limit value fails to convert to uint64:", err)
				return err
			}
			if limitOut == 0 {
				limitOut = 1
			}
			if limitOut == utils.MAX_BT_LIMIT {
				err := TCDeleteTbfContainer(podNic, app.NetNS)
				if err != nil {
					klog.Errorf("delete app %v egress limit error: %v", app.AppName, err)
					return err
				}
			} else if limitOut > 0 {
				err = addQiscInCnet(podNic, app.NetNS, mbitToBit(uint64(limitOut)))
				if err != nil {
					klog.Errorf("failed in addQiscInCnet. Reason: %v", err)
					return err
				}
			}
		}
	} else {
		return fmt.Errorf("app %s group %s doesn't exist", app.AppName, app.Group)
	}
	return nil
}

func NewAdqPlugin(nic, ip string, c CNI, group []Group) (Plugin, string, error) {
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
			groupSettable.Groups = append(groupSettable.Groups, GroupSettable{Id: grp.Id, Settable: false})
		}
	}
	grpSettableStr, err := json.Marshal(groupSettable)
	if err != nil {
		klog.Error("failed to transfer groupSettable to string:", err)
	}
	// todo: adqsetup init or maybe already set it in isadq detect python script
	return &AdqPlugin{plugin: NewCommonPlugin(), cni: c, nic: nic, groups: group}, string(grpSettableStr), nil
}

func (a *AdqPlugin) getGroupType(group string) int {
	for _, grp := range a.groups {
		if group == grp.Id {
			return grp.GroupType
		}
	}
	return -1
}

func (a *AdqPlugin) AddEgressFilter(app *AppInfo) error {
	for _, ip := range app.Devices {
		var tc uint32 = 0
		hostDevice, err := netlink.LinkByName(a.nic)
		if err != nil {
			return err
		}
		groupType := a.getGroupType(app.Group)
		if groupType == GROUP_SEPARATE_LAZY || groupType == GROUP_SEPARATE_DYNAMIC {
			tc = 1
		} else if groupType == GROUP_SHARE {
			tc = 2
		} else {
			return fmt.Errorf("app %s group %s doesn't exist", app.AppName, app.Group)
		}
		egress := &netlink.Flower{
			FilterAttrs: netlink.FilterAttrs{
				LinkIndex: hostDevice.Attrs().Index,
				Parent:    uint32(netlink.HANDLE_MIN_EGRESS),
				Priority:  1,
				Protocol:  unix.ETH_P_IP,
				// Handle:    0,
			},
			EthType: unix.ETH_P_IP,
			SrcIP:   net.ParseIP(ip),
			Actions: []netlink.Action{},
		}
		skbeditPrio := netlink.NewSkbEditAction()
		skbeditPrio.Priority = &tc
		egress.Actions = append(egress.Actions, skbeditPrio)
		if err := netlink.FilterAdd(egress); err != nil {
			klog.Errorf("egress tc filter %v set failed: %v", ip, err)
			return err
		}
	}
	return nil
}

func (a *AdqPlugin) AddIngressFilter(app *AppInfo) error {
	for _, ip := range app.Devices {
		var tc uint32
		hostDevice, err := netlink.LinkByName(a.nic)
		if err != nil {
			return err
		}
		groupType := a.getGroupType(app.Group)
		if groupType == GROUP_SEPARATE_LAZY || groupType == GROUP_SEPARATE_DYNAMIC {
			tc = 1
		} else if groupType == GROUP_SHARE {
			tc = 2
		} else {
			return fmt.Errorf("app %s group %s doesn't exist", app.AppName, app.Group)
		}
		flags := uint32(0)
		flags |= nl.TCA_CLS_FLAGS_SKIP_SW
		maj, min := netlink.MajorMinor(netlink.HANDLE_MIN_PRIORITY + tc)
		handle := netlink.MakeHandle(maj, min)

		ingress := &netlink.Flower{
			FilterAttrs: netlink.FilterAttrs{
				LinkIndex: hostDevice.Attrs().Index,
				Parent:    netlink.HANDLE_INGRESS + 1,
				Priority:  1,
				Protocol:  unix.ETH_P_IP,
				// Handle:    0,
			},
			EthType: unix.ETH_P_IP,
			DestIP:  net.ParseIP(ip),
			Flags:   flags,
			ClassID: handle,
		}
		if err := netlink.FilterAdd(ingress); err != nil {
			klog.Errorf("ingress tc filter set failed: %v", err)
			return err
		}
	}
	return nil
}

func (a *AdqPlugin) DelADQFilter(ip net.IP) error {
	filters, err := a.TCGetFlowerFilters()
	if err != nil {
		return err
	}
	for _, flower := range filters {
		if flower.DestIP.Equal(ip) || flower.SrcIP.Equal(ip) {
			if err := netlink.FilterDel(flower); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *AdqPlugin) TCGetFlowerFilters() ([]*netlink.Flower, error) {
	var rv []*netlink.Flower
	hostDevice, err := netlink.LinkByName(a.nic)
	if err != nil {
		return nil, err
	}
	filters, err := netlink.FilterList(hostDevice, netlink.HANDLE_MIN_EGRESS)
	if err != nil {
		return nil, err
	}
	for _, filter := range filters {
		flower, ok := filter.(*netlink.Flower)
		if !ok {
			continue
		}

		if len(flower.Actions) > 0 {
			_, ok := flower.Actions[0].(*netlink.SkbEditAction)
			if !ok {
				continue
			}
			rv = append(rv, flower)
		}
	}
	filters, err = netlink.FilterList(hostDevice, netlink.HANDLE_MIN_INGRESS)
	if err != nil {
		return nil, err
	}
	for _, filter := range filters {
		flower, ok := filter.(*netlink.Flower)
		if !ok {
			continue
		}
		if flower.ClassID != 0 {
			rv = append(rv, flower)
		}
	}
	return rv, nil
}
