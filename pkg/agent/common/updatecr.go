/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package common

import (
	"context"
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	"sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	utils "sigs.k8s.io/IOIsolation/pkg"
	"sigs.k8s.io/IOIsolation/pkg/agent"
)

func UpdateNodeStaticIOInfo(nodeStaticIOInfo *v1.NodeStaticIOInfo, v versioned.Interface) error {
	err := UpdateNodeStaticIOInfoSpec(nodeStaticIOInfo, v)
	if err != nil {
		klog.Errorf("UpdateNodeStaticIOInfoSpec failed: %v", err)
		return err
	}
	err = UpdateNodeStaticIOInfoStatus(nodeStaticIOInfo, v)
	if err != nil {
		klog.Errorf("UpdateNodeStaticIOInfoStatus failed: %v", err)
		return err
	}

	return nil
}

func CreateNodeStaticIOInfo(nodeStaticIOInfo *v1.NodeStaticIOInfo, v versioned.Interface) error {
	var CrNamespace = utils.CrNamespace
	_, err := v.IoiV1().NodeStaticIOInfos(CrNamespace).Create(context.TODO(), nodeStaticIOInfo, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("CreateNodeStaticIOInfo failed: %v", err)
		return err
	}
	return nil
}

func UpdateNodeStaticIOInfoSpec(nodeStaticIOInfo *v1.NodeStaticIOInfo, v versioned.Interface) error {
	var CrNamespace = utils.CrNamespace
	cur, err := GetNodeStaticIOInfos(v)
	if err != nil {
		err = CreateNodeStaticIOInfo(nodeStaticIOInfo, v)
		if err != nil {
			klog.Errorf("CreateNodeStaticIOInfo failed: %v", err)
			return err
		}
	} else {
		newCur := cur.DeepCopy()
		newCur.Spec = nodeStaticIOInfo.Spec
		_, err := v.IoiV1().NodeStaticIOInfos(CrNamespace).Update(context.TODO(), newCur, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("UpdateNodeStaticIOInfoSpec failed: %v", err)
			return err
		}
	}

	return nil
}

func UpdateNodeStaticIOInfoStatus(nodeStaticIOInfo *v1.NodeStaticIOInfo, v versioned.Interface) error {
	var CrNamespace = utils.CrNamespace
	cur, err := GetNodeStaticIOInfos(v)
	if err != nil {
		err = CreateNodeStaticIOInfo(nodeStaticIOInfo, v)
		if err != nil {
			klog.Errorf("CreateNodeStaticIOInfo failed: %v", err)
			return err
		}
	} else {
		newCur := cur.DeepCopy()
		newCur.Status = nodeStaticIOInfo.Status
		_, err := v.IoiV1().NodeStaticIOInfos(CrNamespace).UpdateStatus(context.TODO(), newCur, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("UpdateNodeStaticIOInfoStatus failed UpdateStatus: %v", err)
			return err
		}
	}

	return nil
}

func GetNodeStaticIOInfos(v versioned.Interface) (*v1.NodeStaticIOInfo, error) {
	if v == nil {
		klog.Error("clientset is nil")
		return nil, errors.New("clientset is nil")
	}
	var CrNamespace = utils.CrNamespace
	c := &agent.GetAgent().IOInfo
	pNodeStaticIOInfo, err := v.IoiV1().NodeStaticIOInfos(CrNamespace).Get(context.TODO(), c.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("GetNodeStaticIOInfos failed: %v", err)
		return nil, err
	}
	return pNodeStaticIOInfo, nil
}
