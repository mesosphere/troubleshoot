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

// Code generated by informer-gen. DO NOT EDIT.

package v1beta1

import (
	"context"
	time "time"

	longhornv1beta1 "github.com/replicatedhq/troubleshoot/pkg/longhorn/apis/longhorn/v1beta1"
	versioned "github.com/replicatedhq/troubleshoot/pkg/longhorn/client/clientset/versioned"
	internalinterfaces "github.com/replicatedhq/troubleshoot/pkg/longhorn/client/informers/externalversions/internalinterfaces"
	v1beta1 "github.com/replicatedhq/troubleshoot/pkg/longhorn/client/listers/longhorn/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// BackingImageDataSourceInformer provides access to a shared informer and lister for
// BackingImageDataSources.
type BackingImageDataSourceInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1beta1.BackingImageDataSourceLister
}

type backingImageDataSourceInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewBackingImageDataSourceInformer constructs a new informer for BackingImageDataSource type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewBackingImageDataSourceInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredBackingImageDataSourceInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredBackingImageDataSourceInformer constructs a new informer for BackingImageDataSource type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredBackingImageDataSourceInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.LonghornV1beta1().BackingImageDataSources(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.LonghornV1beta1().BackingImageDataSources(namespace).Watch(context.TODO(), options)
			},
		},
		&longhornv1beta1.BackingImageDataSource{},
		resyncPeriod,
		indexers,
	)
}

func (f *backingImageDataSourceInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredBackingImageDataSourceInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *backingImageDataSourceInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&longhornv1beta1.BackingImageDataSource{}, f.defaultInformer)
}

func (f *backingImageDataSourceInformer) Lister() v1beta1.BackingImageDataSourceLister {
	return v1beta1.NewBackingImageDataSourceLister(f.Informer().GetIndexer())
}
