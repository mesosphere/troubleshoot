/*
Copyright 2019 Replicated, Inc..

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
// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1beta2 "github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/typed/troubleshoot/v1beta2"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeTroubleshootV1beta2 struct {
	*testing.Fake
}

func (c *FakeTroubleshootV1beta2) Analyzers(namespace string) v1beta2.AnalyzerInterface {
	return &FakeAnalyzers{c, namespace}
}

func (c *FakeTroubleshootV1beta2) Collectors(namespace string) v1beta2.CollectorInterface {
	return &FakeCollectors{c, namespace}
}

func (c *FakeTroubleshootV1beta2) HostPreflights(namespace string) v1beta2.HostPreflightInterface {
	return &FakeHostPreflights{c, namespace}
}

func (c *FakeTroubleshootV1beta2) Preflights(namespace string) v1beta2.PreflightInterface {
	return &FakePreflights{c, namespace}
}

func (c *FakeTroubleshootV1beta2) Redactors(namespace string) v1beta2.RedactorInterface {
	return &FakeRedactors{c, namespace}
}

func (c *FakeTroubleshootV1beta2) SupportBundles(namespace string) v1beta2.SupportBundleInterface {
	return &FakeSupportBundles{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeTroubleshootV1beta2) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
