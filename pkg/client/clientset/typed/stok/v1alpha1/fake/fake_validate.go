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

// FakeValidates implements ValidateInterface
type FakeValidates struct {
	Fake *FakeStokV1alpha1
	ns   string
}

var validatesResource = schema.GroupVersionResource{Group: "stok.goalspike.com", Version: "v1alpha1", Resource: "validates"}

var validatesKind = schema.GroupVersionKind{Group: "stok.goalspike.com", Version: "v1alpha1", Kind: "Validate"}

// Get takes name of the validate, and returns the corresponding validate object, and an error if there is any.
func (c *FakeValidates) Get(name string, options v1.GetOptions) (result *v1alpha1.Validate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(validatesResource, c.ns, name), &v1alpha1.Validate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Validate), err
}

// List takes label and field selectors, and returns the list of Validates that match those selectors.
func (c *FakeValidates) List(opts v1.ListOptions) (result *v1alpha1.ValidateList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(validatesResource, validatesKind, c.ns, opts), &v1alpha1.ValidateList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.ValidateList{ListMeta: obj.(*v1alpha1.ValidateList).ListMeta}
	for _, item := range obj.(*v1alpha1.ValidateList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested validates.
func (c *FakeValidates) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(validatesResource, c.ns, opts))

}

// Create takes the representation of a validate and creates it.  Returns the server's representation of the validate, and an error, if there is any.
func (c *FakeValidates) Create(validate *v1alpha1.Validate) (result *v1alpha1.Validate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(validatesResource, c.ns, validate), &v1alpha1.Validate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Validate), err
}

// Update takes the representation of a validate and updates it. Returns the server's representation of the validate, and an error, if there is any.
func (c *FakeValidates) Update(validate *v1alpha1.Validate) (result *v1alpha1.Validate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(validatesResource, c.ns, validate), &v1alpha1.Validate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Validate), err
}

// Delete takes name of the validate and deletes it. Returns an error if one occurs.
func (c *FakeValidates) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(validatesResource, c.ns, name), &v1alpha1.Validate{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeValidates) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(validatesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.ValidateList{})
	return err
}

// Patch applies the patch and returns the patched validate.
func (c *FakeValidates) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.Validate, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(validatesResource, c.ns, name, pt, data, subresources...), &v1alpha1.Validate{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Validate), err
}
