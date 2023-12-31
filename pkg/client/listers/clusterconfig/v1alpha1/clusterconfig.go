/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/myoperator/clusterconfigoperator/pkg/apis/clusterconfig/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// ClusterConfigLister helps list ClusterConfigs.
// All objects returned here must be treated as read-only.
type ClusterConfigLister interface {
	// List lists all ClusterConfigs in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.ClusterConfig, err error)
	// ClusterConfigs returns an object that can list and get ClusterConfigs.
	ClusterConfigs(namespace string) ClusterConfigNamespaceLister
	ClusterConfigListerExpansion
}

// clusterConfigLister implements the ClusterConfigLister interface.
type clusterConfigLister struct {
	indexer cache.Indexer
}

// NewClusterConfigLister returns a new ClusterConfigLister.
func NewClusterConfigLister(indexer cache.Indexer) ClusterConfigLister {
	return &clusterConfigLister{indexer: indexer}
}

// List lists all ClusterConfigs in the indexer.
func (s *clusterConfigLister) List(selector labels.Selector) (ret []*v1alpha1.ClusterConfig, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.ClusterConfig))
	})
	return ret, err
}

// ClusterConfigs returns an object that can list and get ClusterConfigs.
func (s *clusterConfigLister) ClusterConfigs(namespace string) ClusterConfigNamespaceLister {
	return clusterConfigNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// ClusterConfigNamespaceLister helps list and get ClusterConfigs.
// All objects returned here must be treated as read-only.
type ClusterConfigNamespaceLister interface {
	// List lists all ClusterConfigs in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.ClusterConfig, err error)
	// Get retrieves the ClusterConfig from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.ClusterConfig, error)
	ClusterConfigNamespaceListerExpansion
}

// clusterConfigNamespaceLister implements the ClusterConfigNamespaceLister
// interface.
type clusterConfigNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all ClusterConfigs in the indexer for a given namespace.
func (s clusterConfigNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.ClusterConfig, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.ClusterConfig))
	})
	return ret, err
}

// Get retrieves the ClusterConfig from the indexer for a given namespace and name.
func (s clusterConfigNamespaceLister) Get(name string) (*v1alpha1.ClusterConfig, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("clusterconfig"), name)
	}
	return obj.(*v1alpha1.ClusterConfig), nil
}
