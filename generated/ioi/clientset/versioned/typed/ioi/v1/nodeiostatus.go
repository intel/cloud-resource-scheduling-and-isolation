/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/
// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
	v1 "sigs.k8s.io/IOIsolation/api/ioi/v1"
	scheme "sigs.k8s.io/IOIsolation/generated/ioi/clientset/versioned/scheme"
)

// NodeIOStatusesGetter has a method to return a NodeIOStatusInterface.
// A group's client should implement this interface.
type NodeIOStatusesGetter interface {
	NodeIOStatuses(namespace string) NodeIOStatusInterface
}

// NodeIOStatusInterface has methods to work with NodeIOStatus resources.
type NodeIOStatusInterface interface {
	Create(ctx context.Context, nodeIOStatus *v1.NodeIOStatus, opts metav1.CreateOptions) (*v1.NodeIOStatus, error)
	Update(ctx context.Context, nodeIOStatus *v1.NodeIOStatus, opts metav1.UpdateOptions) (*v1.NodeIOStatus, error)
	UpdateStatus(ctx context.Context, nodeIOStatus *v1.NodeIOStatus, opts metav1.UpdateOptions) (*v1.NodeIOStatus, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.NodeIOStatus, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1.NodeIOStatusList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.NodeIOStatus, err error)
	NodeIOStatusExpansion
}

// nodeIOStatuses implements NodeIOStatusInterface
type nodeIOStatuses struct {
	client rest.Interface
	ns     string
}

// newNodeIOStatuses returns a NodeIOStatuses
func newNodeIOStatuses(c *IoiV1Client, namespace string) *nodeIOStatuses {
	return &nodeIOStatuses{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the nodeIOStatus, and returns the corresponding nodeIOStatus object, and an error if there is any.
func (c *nodeIOStatuses) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.NodeIOStatus, err error) {
	result = &v1.NodeIOStatus{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("nodeiostatuses").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of NodeIOStatuses that match those selectors.
func (c *nodeIOStatuses) List(ctx context.Context, opts metav1.ListOptions) (result *v1.NodeIOStatusList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.NodeIOStatusList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("nodeiostatuses").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested nodeIOStatuses.
func (c *nodeIOStatuses) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("nodeiostatuses").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a nodeIOStatus and creates it.  Returns the server's representation of the nodeIOStatus, and an error, if there is any.
func (c *nodeIOStatuses) Create(ctx context.Context, nodeIOStatus *v1.NodeIOStatus, opts metav1.CreateOptions) (result *v1.NodeIOStatus, err error) {
	result = &v1.NodeIOStatus{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("nodeiostatuses").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(nodeIOStatus).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a nodeIOStatus and updates it. Returns the server's representation of the nodeIOStatus, and an error, if there is any.
func (c *nodeIOStatuses) Update(ctx context.Context, nodeIOStatus *v1.NodeIOStatus, opts metav1.UpdateOptions) (result *v1.NodeIOStatus, err error) {
	result = &v1.NodeIOStatus{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("nodeiostatuses").
		Name(nodeIOStatus.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(nodeIOStatus).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *nodeIOStatuses) UpdateStatus(ctx context.Context, nodeIOStatus *v1.NodeIOStatus, opts metav1.UpdateOptions) (result *v1.NodeIOStatus, err error) {
	result = &v1.NodeIOStatus{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("nodeiostatuses").
		Name(nodeIOStatus.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(nodeIOStatus).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the nodeIOStatus and deletes it. Returns an error if one occurs.
func (c *nodeIOStatuses) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("nodeiostatuses").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *nodeIOStatuses) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("nodeiostatuses").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched nodeIOStatus.
func (c *nodeIOStatuses) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.NodeIOStatus, err error) {
	result = &v1.NodeIOStatus{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("nodeiostatuses").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
