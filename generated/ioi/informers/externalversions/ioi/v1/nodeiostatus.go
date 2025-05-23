/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/
// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	"context"
	time "time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
	ioiv1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	versioned "sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned"
	internalinterfaces "sigs.k8s.io/IOIsolation/generated/ioi/informers/externalversions/internalinterfaces"
	v1 "sigs.k8s.io/IOIsolation/generated/ioi/listers/ioi/v1"
)

// NodeIOStatusInformer provides access to a shared informer and lister for
// NodeIOStatuses.
type NodeIOStatusInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.NodeIOStatusLister
}

type nodeIOStatusInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewNodeIOStatusInformer constructs a new informer for NodeIOStatus type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewNodeIOStatusInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredNodeIOStatusInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredNodeIOStatusInformer constructs a new informer for NodeIOStatus type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredNodeIOStatusInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.IoiV1().NodeIOStatuses(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.IoiV1().NodeIOStatuses(namespace).Watch(context.TODO(), options)
			},
		},
		&ioiv1.NodeIOStatus{},
		resyncPeriod,
		indexers,
	)
}

func (f *nodeIOStatusInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredNodeIOStatusInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *nodeIOStatusInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&ioiv1.NodeIOStatus{}, f.defaultInformer)
}

func (f *nodeIOStatusInformer) Lister() v1.NodeIOStatusLister {
	return v1.NewNodeIOStatusLister(f.Informer().GetIndexer())
}
