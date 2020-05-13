// Copyright © 2020 Louis Garman <louisgarman@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeUntaints implements UntaintInterface
type FakeUntaints struct {
	Fake *FakeStokV1alpha1
	ns   string
}

var untaintsResource = schema.GroupVersionResource{Group: "stok.goalspike.com", Version: "v1alpha1", Resource: "untaints"}

var untaintsKind = schema.GroupVersionKind{Group: "stok.goalspike.com", Version: "v1alpha1", Kind: "Untaint"}

// Get takes name of the untaint, and returns the corresponding untaint object, and an error if there is any.
func (c *FakeUntaints) Get(name string, options v1.GetOptions) (result *v1alpha1.Untaint, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(untaintsResource, c.ns, name), &v1alpha1.Untaint{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Untaint), err
}

// List takes label and field selectors, and returns the list of Untaints that match those selectors.
func (c *FakeUntaints) List(opts v1.ListOptions) (result *v1alpha1.UntaintList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(untaintsResource, untaintsKind, c.ns, opts), &v1alpha1.UntaintList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.UntaintList{ListMeta: obj.(*v1alpha1.UntaintList).ListMeta}
	for _, item := range obj.(*v1alpha1.UntaintList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested untaints.
func (c *FakeUntaints) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(untaintsResource, c.ns, opts))

}

// Create takes the representation of a untaint and creates it.  Returns the server's representation of the untaint, and an error, if there is any.
func (c *FakeUntaints) Create(untaint *v1alpha1.Untaint) (result *v1alpha1.Untaint, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(untaintsResource, c.ns, untaint), &v1alpha1.Untaint{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Untaint), err
}

// Update takes the representation of a untaint and updates it. Returns the server's representation of the untaint, and an error, if there is any.
func (c *FakeUntaints) Update(untaint *v1alpha1.Untaint) (result *v1alpha1.Untaint, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(untaintsResource, c.ns, untaint), &v1alpha1.Untaint{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Untaint), err
}

// Delete takes name of the untaint and deletes it. Returns an error if one occurs.
func (c *FakeUntaints) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(untaintsResource, c.ns, name), &v1alpha1.Untaint{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeUntaints) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(untaintsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.UntaintList{})
	return err
}

// Patch applies the patch and returns the patched untaint.
func (c *FakeUntaints) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.Untaint, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(untaintsResource, c.ns, name, pt, data, subresources...), &v1alpha1.Untaint{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Untaint), err
}
