/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package net

import (
	"errors"
	"fmt"
	"net"
	"syscall"

	utils "sigs.k8s.io/IOIsolation/pkg"

	"github.com/containerd/go-cni"
	"github.com/vishvananda/netlink"

	"github.com/containernetworking/plugins/pkg/ns"
	"k8s.io/klog/v2"
)

const (
	CniConfDir              = "/etc/cni/net.d"
	NetworkPluginMaxConfNum = 1
	NetworkAttachCount      = 2
	FlannelUDP              = "flannel0"
	FlannelVxlan            = "flannel.1"
	CalicoVxlan             = "vxlan.calico"
	CiliumHost              = "cilium_host"
	CiliumVxlan             = "cilium_vxlan"
	Overlay                 = "overlay"
	NonOverlay              = ""
)

func GetInterfaceIpv4AddrInNamespace(interfaceName, namespace string) (addr string, err error) {
	var (
		ief      *net.Interface
		addrs    []net.Addr
		ipv4Addr net.IP
	)
	netns, err := ns.GetNS(namespace)
	if err != nil {
		return "", err
	}
	defer func() {
		closeErr := netns.Close()
		if err == nil {
			err = closeErr
		}
	}()
	err = netns.Do(func(_ ns.NetNS) error {
		if ief, err = net.InterfaceByName(interfaceName); err != nil { // get interface
			return err
		}

		if addrs, err = ief.Addrs(); err != nil { // get addresses
			return err
		}
		for _, addr := range addrs { // get ipv4 address
			if ipv4Addr = addr.(*net.IPNet).IP.To4(); ipv4Addr != nil {
				break
			}
		}
		if ipv4Addr == nil {
			return fmt.Errorf(fmt.Sprintf("interface %s don't have an ipv4 address\n", interfaceName))
		}
		return nil
	})
	klog.V(utils.INF).Infof("ip address is %s", ipv4Addr.String())
	return ipv4Addr.String(), err
}

func GetDefaultGatewayInterface() (*net.Interface, error) {
	routes, err := netlink.RouteList(nil, syscall.AF_INET)
	if err != nil {
		return nil, err
	}

	for _, route := range routes {
		if route.Dst == nil || route.Dst.String() == "0.0.0.0/0" {
			if route.LinkIndex <= 0 {
				return nil, errors.New("found default route but could not determine interface")
			}
			return net.InterfaceByIndex(route.LinkIndex)
		}
	}

	return nil, errors.New("unable to find default route")
}

func GetDefaultGatewayInterfaceInNetNS(namespace string) (*net.Interface, error) {
	if namespace == "" {
		return GetDefaultGatewayInterface()
	}
	var ief *net.Interface

	netns, err := ns.GetNS(namespace)
	if err != nil {
		return nil, err
	}
	defer func() {
		closeErr := netns.Close()
		if err == nil {
			err = closeErr
		}
	}()

	err = netns.Do(func(_ ns.NetNS) error {
		routes, err := netlink.RouteList(nil, syscall.AF_INET)
		if err != nil {
			return err
		}

		for _, route := range routes {
			if route.Dst == nil || route.Dst.String() == "0.0.0.0/0" {
				if route.LinkIndex <= 0 {
					return errors.New("found default route but could not determine interface")
				}
				ief, err = net.InterfaceByIndex(route.LinkIndex)
				if err != nil {
					return err
				}
				return nil
			}
		}
		return errors.New("unable to find default route")
	})

	return ief, err
}

func DetectCNI() (string, error) {
	i, err := cni.New(cni.WithMinNetworkCount(NetworkAttachCount),
		cni.WithPluginConfDir(CniConfDir),
		cni.WithPluginMaxConfNum(NetworkPluginMaxConfNum))
	if err != nil {
		return "", fmt.Errorf("failed to initialize cni: %w", err)
	}

	if err := i.Load(cni.WithLoNetwork, cni.WithDefaultConf); err != nil {
		return "", fmt.Errorf("failed to load cni configuration: %v", err)
	}
	if nil != i.GetConfig() && len(i.GetConfig().Networks) > 1 && i.GetConfig().Networks[1].Config != nil && len(i.GetConfig().Networks[1].Config.Plugins) > 0 && i.GetConfig().Networks[1].Config.Plugins[0].Network != nil {
		return i.GetConfig().Networks[1].Config.Plugins[0].Network.Type, nil
	}
	return "", fmt.Errorf("failed to get cni, %v", i.GetConfig())
}

func DetectBackendFlannel() (string, error) {
	var backend string
	routes, err := netlink.RouteList(nil, syscall.AF_INET)
	if err != nil {
		return "", err
	}

	for _, route := range routes {
		iface, err := net.InterfaceByIndex(route.LinkIndex)
		if err != nil {
			klog.Errorf("could not get interface by route index: %v", err)
			continue
		}
		if iface.Name == FlannelVxlan {
			if route.LinkIndex <= 0 {
				return "", errors.New("could not determine backend by interface")
			}
			backend = iface.Name
			return backend, nil
		}
	}

	backend = FlannelUDP
	return backend, nil
}

func DetectBackendCalico() (string, error) {
	var backend string
	routes, err := netlink.RouteList(nil, syscall.AF_INET)
	if err != nil {
		return "", err
	}

	for _, route := range routes {
		iface, err := net.InterfaceByIndex(route.LinkIndex)
		if err != nil {
			klog.Errorf("could not get interface by route index: %v", err)
			continue
		}
		if iface.Name == CalicoVxlan {
			if route.LinkIndex <= 0 {
				return "", errors.New("could not determine backend by interface")
			}
			backend = iface.Name
			return backend, nil
		}
	}

	backend = NonOverlay
	return backend, nil
}

func DetectBackendCilium() (string, error) {
	var backend string
	backend = CiliumHost
	links, _ := netlink.LinkList()
	for _, link := range links {
		if link.Attrs().Name == CiliumVxlan {
			backend = link.Attrs().Name
		}
	}
	return backend, nil
}
