/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/
// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
)

// NodeStaticIOInfoLister helps list NodeStaticIOInfos.
// All objects returned here must be treated as read-only.
type NodeStaticIOInfoLister interface {
	// List lists all NodeStaticIOInfos in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.NodeStaticIOInfo, err error)
	// NodeStaticIOInfos returns an object that can list and get NodeStaticIOInfos.
	NodeStaticIOInfos(namespace string) NodeStaticIOInfoNamespaceLister
	NodeStaticIOInfoListerExpansion
}

// nodeStaticIOInfoLister implements the NodeStaticIOInfoLister interface.
type nodeStaticIOInfoLister struct {
	indexer cache.Indexer
}

// NewNodeStaticIOInfoLister returns a new NodeStaticIOInfoLister.
func NewNodeStaticIOInfoLister(indexer cache.Indexer) NodeStaticIOInfoLister {
	return &nodeStaticIOInfoLister{indexer: indexer}
}

// List lists all NodeStaticIOInfos in the indexer.
func (s *nodeStaticIOInfoLister) List(selector labels.Selector) (ret []*v1.NodeStaticIOInfo, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.NodeStaticIOInfo))
	})
	return ret, err
}

// NodeStaticIOInfos returns an object that can list and get NodeStaticIOInfos.
func (s *nodeStaticIOInfoLister) NodeStaticIOInfos(namespace string) NodeStaticIOInfoNamespaceLister {
	return nodeStaticIOInfoNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// NodeStaticIOInfoNamespaceLister helps list and get NodeStaticIOInfos.
// All objects returned here must be treated as read-only.
type NodeStaticIOInfoNamespaceLister interface {
	// List lists all NodeStaticIOInfos in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.NodeStaticIOInfo, err error)
	// Get retrieves the NodeStaticIOInfo from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1.NodeStaticIOInfo, error)
	NodeStaticIOInfoNamespaceListerExpansion
}

// nodeStaticIOInfoNamespaceLister implements the NodeStaticIOInfoNamespaceLister
// interface.
type nodeStaticIOInfoNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all NodeStaticIOInfos in the indexer for a given namespace.
func (s nodeStaticIOInfoNamespaceLister) List(selector labels.Selector) (ret []*v1.NodeStaticIOInfo, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.NodeStaticIOInfo))
	})
	return ret, err
}

// Get retrieves the NodeStaticIOInfo from the indexer for a given namespace and name.
func (s nodeStaticIOInfoNamespaceLister) Get(name string) (*v1.NodeStaticIOInfo, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1.Resource("nodestaticioinfo"), name)
	}
	return obj.(*v1.NodeStaticIOInfo), nil
}
