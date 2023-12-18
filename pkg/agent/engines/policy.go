/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package engines

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"strings"

	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	"sigs.k8s.io/IOIsolation/pkg/agent"
)

var (
	TypePolicy = "Policy"
	PolicyName = "policy-configmap"
)

// PolicyEngine
type PolicyEngine struct {
}

func (p *PolicyEngine) Type() string {
	return TypePolicy
}

func (p *PolicyEngine) Initialize(coreClient *kubernetes.Clientset, client *versioned.Clientset, mtls bool) error {
	klog.Info("3.3 initializing the policy engine")
	pFactory := p.CreatePolicyFactory()
	if pFactory != nil {
		pFactory.Start(wait.NeverStop)
		pFactory.WaitForCacheSync(wait.NeverStop)
	}

	return nil
}

func (p *PolicyEngine) CreatePolicyFactory() informers.SharedInformerFactory {
	coreFactory := informers.NewSharedInformerFactory(agent.GetAgent().CoreClient, 0)
	coreInformer := coreFactory.Core().V1().ConfigMaps().Informer()

	handler := cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			pl, ok := obj.(*corev1.ConfigMap)
			if !ok || pl.Name != PolicyName {
				return false
			}
			return true
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    p.AddPolicyConfigData,
			UpdateFunc: p.UpdatePolicyConfigData,
		},
	}
	if _, err := coreInformer.AddEventHandler(handler); err != nil {
		klog.Error("failed to bind the events to ConfigMaps")
		return nil
	}

	return coreFactory
}

func (p *PolicyEngine) AddPolicyConfigData(obj interface{}) {
	pl, _ := obj.(*corev1.ConfigMap)
	data := p.AnalysisPolicyConfig(pl)
	agent.PolicyChan <- data
}

func (p *PolicyEngine) UpdatePolicyConfigData(oldObj, newObj interface{}) {
	pl, _ := newObj.(*corev1.ConfigMap)
	data := p.AnalysisPolicyConfig(pl)
	agent.PolicyChan <- data
}

func (p *PolicyEngine) AnalysisPolicyConfig(pl *corev1.ConfigMap) *agent.PolicyConfig {
	var policyConfig agent.PolicyConfig
	diskGroup := pl.Data["DiskGroup"]
	netGroup := pl.Data["NetGroup"]
	diskconfig := p.ParsePolicy(pl, diskGroup)
	netconfig := p.ParsePolicy(pl, netGroup)
	policyConfig = make(map[string]agent.Policy)

	for key, value := range diskconfig {
		policyConfig[key] = value
	}
	for key, value := range netconfig {
		policyConfig[key] = value
	}

	klog.Infof("parsed policy config: %v", policyConfig)
	return &policyConfig
}

func (p *PolicyEngine) ParsePolicy(pl *corev1.ConfigMap, group string) agent.PolicyConfig {
	var po agent.Policy
	poCon := make(map[string]agent.Policy)
	var result agent.PolicyConfig
	split := strings.Split(string(group), "\n")
	splitLen := len(split) - 1
	for i := 0; i < splitLen; i++ {
		tmp := strings.Split(split[i], ":")
		name := tmp[0]
		policy := tmp[1]
		v, ok := pl.Data[policy]
		if ok && v != "" {
			line := strings.Split(v, "\n")
			con := [5][]string{}
			for j := 0; j < len(line)-1; j++ {
				con[j] = strings.Split(line[j], ":")
			}
			po.Throttle = con[0][1]
			po.Select_method = con[1][1]
			po.Compress_method = con[2][1]
			po.Decompress_method = con[3][1]
			po.Report_Flag = con[4][1]
		}
		poCon[name] = po
	}
	result = poCon
	return result
}

// Uninitialize
func (p *PolicyEngine) Uninitialize() error {
	return nil
}

func init() {
	engine := &PolicyEngine{}
	agent.GetAgent().RegisterEngine(engine)
}
