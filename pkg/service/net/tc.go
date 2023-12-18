/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package net

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"k8s.io/klog/v2"
	utils "sigs.k8s.io/IOIsolation/pkg"
)

const latencyInMillis = 25

func getMTU(deviceName string) (int, error) {
	link, err := netlink.LinkByName(deviceName)
	if err != nil {
		return -1, err
	}
	return link.Attrs().MTU, nil
}

func createIfb(ifbDeviceName string, mtu int) error {
	// delete if exist
	ls, err := netlink.LinkList()
	if err != nil {
		return err
	}
	for _, lk := range ls {
		if lk.Attrs().Name == ifbDeviceName {
			link, err := netlink.LinkByName(ifbDeviceName)
			if err != nil {
				return err
			}
			err = netlink.LinkDel(link)
			if err != nil {
				return err
			}
			break
		}
	}
	// create
	err = netlink.LinkAdd(&netlink.Ifb{
		LinkAttrs: netlink.LinkAttrs{
			Name:  ifbDeviceName,
			Flags: net.FlagUp,
			MTU:   mtu,
		},
	})
	if err != nil {
		return err
	}
	// launch
	ifb, err := netlink.LinkByName(ifbDeviceName)
	if err != nil {
		return err
	}
	if err = netlink.LinkSetUp(ifb); err != nil {
		return err
	}
	return nil
}

func addIfbLimit(bw uint64, ifb string) error {
	klog.V(utils.DBG).Info("ifb name:", ifb)
	ifbDevice, err := netlink.LinkByName(ifb)
	if err != nil {
		return err
	}
	// throttle traffic on ifb device
	if err := createTBF(bw, mbitToBit(8), ifbDevice.Attrs().Index); err != nil {
		return err
	}
	return nil
}

func delQdiscIfExist(linkIndex int, tpe string) error {
	var isType bool
	lk, err := netlink.LinkByIndex(linkIndex)
	if err != nil {
		return err
	}
	qds, err := netlink.QdiscList(lk)
	if err != nil {
		return err
	}
	for _, qd := range qds {
		if tpe == "tbf" {
			_, isType = qd.(*netlink.Tbf)
		}
		if tpe == "ingress" {
			_, isType = qd.(*netlink.Ingress)
		}
		if qd.Attrs().LinkIndex == linkIndex && isType {
			err := netlink.QdiscDel(qd)
			if err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func createTBF(rateInBits, burstInBits uint64, linkIndex int) error {
	if rateInBits <= 0 || burstInBits <= 0 {
		return fmt.Errorf("invalid rate or burst: %d, %d", rateInBits, burstInBits)
	}
	rateInBytes := rateInBits / 8
	burstInBytes := burstInBits / 8
	bufferInBytes := buffer(uint64(rateInBytes), uint32(burstInBytes))
	latency := latencyInUsec(latencyInMillis)
	limitInBytes := limit(uint64(rateInBytes), latency, uint32(burstInBytes))

	qdisc := &netlink.Tbf{
		QdiscAttrs: netlink.QdiscAttrs{
			LinkIndex: linkIndex,
			Handle:    netlink.MakeHandle(1, 0),
			Parent:    netlink.HANDLE_ROOT,
		},
		Limit:  uint32(limitInBytes),
		Rate:   uint64(rateInBytes),
		Buffer: uint32(bufferInBytes),
	}
	// delete if exist
	err := delQdiscIfExist(linkIndex, "tbf")
	if err != nil {
		return err
	}
	if err := netlink.QdiscAdd(qdisc); err != nil {
		return err
	}
	return nil
}

func time2Tick(time uint32) uint32 {
	return uint32(float64(time) * netlink.TickInUsec())
}

func buffer(rate uint64, burst uint32) uint32 {
	return time2Tick(uint32(float64(burst) * float64(netlink.TIME_UNITS_PER_SEC) / float64(rate)))
}

func limit(rate uint64, latency float64, buffer uint32) uint32 {
	return uint32(float64(rate)*latency/float64(netlink.TIME_UNITS_PER_SEC)) + buffer
}

func latencyInUsec(latencyInMillis float64) float64 {
	return float64(netlink.TIME_UNITS_PER_SEC) * (latencyInMillis / 1000.0)
}

func getHostInterface(containerIfName string, namespace string) (netlink.Link, error) {
	i := 0
	for {
		klog.Infof("%d round getHostInterface", i)
		i++
		var peerIndex int
		// get namespace
		netns, err := ns.GetNS(namespace)
		if err != nil {
			return nil, err
		}
		// get veth peer index of container interface
		defer netns.Close()
		_ = netns.Do(func(_ ns.NetNS) error {
			_, peerIndex, err = ip.GetVethPeerIfindex(containerIfName)
			return nil
		})
		if peerIndex <= 0 {
			time.Sleep(5 * time.Second)
			klog.Errorf("container interface %s has no veth peer: %v, netns is %s", containerIfName, err, namespace)
		}
		// find host interface by index
		link, err := netlink.LinkByIndex(peerIndex)
		if err != nil {
			time.Sleep(5 * time.Second)
			klog.Errorf("veth peer with index %d is not in host ns", peerIndex)
		} else {
			return link, nil
		}
	}
}

func operateIngressQdisc(hostDevice netlink.Link, op int32) error {
	// add qdisc ingress on host device
	ingress := &netlink.Ingress{
		QdiscAttrs: netlink.QdiscAttrs{
			LinkIndex: hostDevice.Attrs().Index,
			Handle:    netlink.MakeHandle(0xffff, 0), // ffff:
			Parent:    netlink.HANDLE_INGRESS,
		},
	}
	if op == OP_ADD {
		err := delQdiscIfExist(hostDevice.Attrs().Index, "ingress")
		if err != nil {
			return err
		}
		err = netlink.QdiscAdd(ingress)
		if err != nil {
			return err
		}
	} else if op == OP_DEL {
		err := netlink.QdiscDel(ingress)
		if err != nil {
			return err
		}
	}
	return nil
}

func operateFilter(ip uint32, IPoff int32, hostDevice netlink.Link, ifbDevice netlink.Link, op int32) error {
	// var err error
	podsIP := netlink.TcU32Key{
		Val:  ip,
		Mask: 0xFFFFFFFF,
		Off:  IPoff,
	}
	filter := &netlink.U32{
		FilterAttrs: netlink.FilterAttrs{
			LinkIndex: hostDevice.Attrs().Index,
			Parent:    netlink.MakeHandle(0xffff, 0),
			Priority:  1,
			Protocol:  syscall.ETH_P_ALL,
		},
		ClassId:    netlink.MakeHandle(1, 1),
		RedirIndex: ifbDevice.Attrs().Index,
		Sel: &netlink.TcU32Sel{
			Flags: nl.TC_U32_TERMINAL,
			Keys:  []netlink.TcU32Key{podsIP},
		},
	}
	if op == OP_ADD {
		err := netlink.FilterAdd(filter)
		if err != nil {
			return err
		}
	} else if op == OP_DEL {
		u32f, err := getFilterByIP(hostDevice, ip, IPoff, filter.FilterAttrs.Parent)
		if err != nil {
			return err
		}
		err = netlink.FilterDel(u32f)
		if err != nil {
			return err
		}
	}
	return nil
}

func getFilterByIP(hostDevice netlink.Link, ip uint32, ipOff int32, parent uint32) (*netlink.U32, error) {
	filters, err := netlink.FilterList(hostDevice, parent)
	if err != nil {
		return nil, err
	}
	for _, filter := range filters {
		u32f, ok := filter.(*netlink.U32)
		if !ok {
			continue
		}
		if u32f.Sel.Keys[0].Val == ip && u32f.Sel.Keys[0].Off == ipOff {
			return u32f, nil
		}
	}
	return nil, fmt.Errorf("getFilterByIP failed")
}

func convertIPToUint32(ip_str string) (uint32, error) {
	var ip_opt uint32
	ip := net.ParseIP(ip_str)
	if ip == nil {
		return 0, fmt.Errorf("parsing IP error")
	}
	if err := binary.Read(bytes.NewBuffer(ip.To4()), binary.BigEndian, &ip_opt); err != nil {
		return 0, fmt.Errorf("parsing IP error: %v", err)
	}
	return ip_opt, nil
}

func mbitToBit(bw uint64) uint64 {
	return uint64(utils.MB) * bw
}

func addQiscInCnet(conif string, namespace string, limit uint64) error {
	var err error
	var veth netlink.Link
	// get namespace
	netns, err := ns.GetNS(namespace)
	defer func() {
		closeErr := netns.Close()
		if err == nil {
			err = closeErr
		}
	}()
	_ = netns.Do(func(_ ns.NetNS) error {
		// add qdisc for eth0
		veth, err = netlink.LinkByName(conif)
		if err != nil {
			return err
		}
		err = createTBF(limit, mbitToBit(8), veth.Attrs().Index)
		return nil
	})
	return err
}

func operateApp(app *AppInfo, op int32, HostNic string) error {
	var podNic string
	var podIP string
	var ifbDevice netlink.Link
	var err error
	hostDevices := [2]netlink.Link{}
	IPoffs := [2]int32{INGRESS_IP_OFF, EGRESS_IP_OFF}
	ifbDeviceNames := [2]string{INGRESS_IFB_PREFIX + "_" + app.Group, EGRESS_IFB_PREFIX + "_" + app.Group}

	// TODO: fixme
	for k, v := range app.GetDevices(true) { // a nic for a pod
		podNic = k
		podIP = v
		break
	}
	klog.V(utils.DBG).Info("podNic, podIP: ", podNic, podIP)
	// get ingress host device and ifb device
	hd, err := netlink.LinkByName(HostNic)
	if err != nil {
		klog.Errorf("failed in LinkByName. Reason: %v", err)
		return err
	}
	hostDevices[0] = hd

	// get egress host device and ifb device
	hd, err = getHostInterface(podNic, app.NetNS)
	if hd == nil {
		// the pod may have been deleted
		klog.Errorf("failed in getHostInterface. Reason: %v", err)
	} else {
		hostDevices[1] = hd
		// Egress: add/del qdisc ingress for host device
		if err = operateIngressQdisc(hostDevices[1], op); err != nil {
			klog.Errorf("failed in operateIngressQdisc. Reason: %v", err)
			return err
		}
	}
	klog.V(utils.DBG).Info("hostDevices: ", hostDevices)

	for idx := range hostDevices {
		if op == OP_ADD || (op == OP_DEL && idx == 0) { // delete Ingress filter only
			hostDevice := hostDevices[idx]
			IPoff := IPoffs[idx]
			// get ifb device
			if ifbDevice, err = netlink.LinkByName(ifbDeviceNames[idx]); err != nil {
				return err
			}
			// add/del filter on host device to mirror traffic to ifb device
			podIPUint32, err := convertIPToUint32(podIP)
			if err != nil {
				klog.Errorf("failed in convertIPToUint32. Reason: %v", err)
				return err
			}
			if err = operateFilter(podIPUint32, IPoff, hostDevice, ifbDevice, op); err != nil {
				klog.Errorf("failed in operateFilter. Reason: %v", err)
				return err
			}
		}
	}
	return nil
}

func operateGALimit(app *AppInfo, limitIn float64, limitOut float64, op int32) error {
	var podNic string
	for k := range app.GetDevices(true) { // a nic for a pod
		podNic = k
		break
	}
	/* for ingress */
	// get host interface
	veth, err := getHostInterface(podNic, app.NetNS)
	if veth == nil {
		klog.Infof("Can not get in getHostInterface. Reason: %v", err)
		return nil
	}

	if op == OP_ADD && limitIn != utils.MAX_BT_LIMIT {
		// add qdisc for host interface
		if limitIn > 0 {
			if err = createTBF(mbitToBit(uint64(limitIn)), mbitToBit(8), veth.Attrs().Index); err != nil {
				klog.Errorf("failed to create tbf. Reason: %v", err)
				return err
			}
		}
	}
	if op == OP_ADD && limitOut != utils.MAX_BT_LIMIT {
		/* for egress */
		// add qdisc in container network
		if limitOut > 0 {
			if err := addQiscInCnet(podNic, app.NetNS, mbitToBit(uint64(limitOut))); err != nil {
				klog.Errorf("failed in operateQiscInCnet. Reason: %v", err)
				return err
			}
		}
	}
	if op == OP_DEL || (op == OP_ADD && limitIn == utils.MAX_BT_LIMIT) {
		// for ingress, delete qdisc for host interface
		if err := TCDeleteTbfHost(podNic, app.NetNS); err != nil {
			klog.Errorf("failed to delete tbf. Reason: %v", err)
			return err
		}

	}
	if op == OP_DEL || (op == OP_ADD && limitOut == utils.MAX_BT_LIMIT) {
		// for egress, add qdisc in container network
		err := TCDeleteTbfContainer(podNic, app.NetNS)
		if err != nil {
			klog.Errorf("failed in operateQiscInCnet. Reason: %v", err)
			return err
		}
	}
	return nil
}

func HasIngress(nic string) (bool, error) {
	var rv []netlink.Qdisc
	eth, err := netlink.LinkByName(nic)
	if err != nil {
		klog.Errorf("failed in getting eth. Reason: %v", err)
		return false, err
	}
	qdiscs, err := netlink.QdiscList(eth)
	if err != nil {
		return false, err
	}
	for _, qdisc := range qdiscs {
		ingress, ok := qdisc.(*netlink.Ingress)
		if !ok {
			continue
		}
		rv = append(rv, ingress)
	}
	return len(rv) > 0, nil
}

func TCDeleteTbfHost(podNic, ns string) error {
	var rv []netlink.Qdisc
	veth, err := getHostInterface(podNic, ns)
	if err != nil {
		klog.Errorf("failed in getHostInterface. Reason: %v", err)
		return nil
	}
	qdiscs, err := netlink.QdiscList(veth)
	if err != nil {
		return err
	}
	for _, qdisc := range qdiscs {
		tbf, ok := qdisc.(*netlink.Tbf)
		if !ok {
			continue
		}
		rv = append(rv, tbf)
	}
	for _, q := range rv {
		err := netlink.QdiscDel(q)
		if err != nil {
			return err
		}
	}
	return nil
}

func TCDeleteTbfContainer(podNic, namespace string) error {
	var veth netlink.Link
	// get namespace
	netns, err := ns.GetNS(namespace)
	if netns == nil {
		klog.Warning("TCDeleteTbfContainer do nothing due to the pod has been deleted")
		return nil
	}
	defer func() {
		closeErr := netns.Close()
		if err == nil {
			err = closeErr
		}
	}()
	err = netns.Do(func(_ ns.NetNS) error {
		// add qdisc for eth0
		var rv []netlink.Qdisc
		veth, err = netlink.LinkByName(podNic)
		if err != nil {
			klog.Errorf("failed in getting veth. Reason: %v", err)
			return err
		}
		qdiscs, err := netlink.QdiscList(veth)
		if err != nil {
			return err
		}
		for _, qdisc := range qdiscs {
			tbf, ok := qdisc.(*netlink.Tbf)
			if !ok {
				continue
			}
			rv = append(rv, tbf)
		}
		for _, q := range rv {
			err := netlink.QdiscDel(q)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
