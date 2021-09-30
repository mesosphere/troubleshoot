package collect

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path" // this code uses 'path' and not 'path/filepath' because we don't want backslashes on windows
	"strings"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1beta1clientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func ClusterResources(c *Collector, clusterResourcesCollector *troubleshootv1beta2.ClusterResources) (CollectorResult, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	output := NewResult()

	// namespaces
	var namespaceNames []string
	if len(clusterResourcesCollector.Namespaces) > 0 {
		namespaces, namespaceErrors := getNamespaces(ctx, client, clusterResourcesCollector.Namespaces)
		namespaceNames = clusterResourcesCollector.Namespaces
		output.SaveResult(c.BundlePath, "cluster-resources/namespaces.json", bytes.NewBuffer(namespaces))
		output.SaveResult(c.BundlePath, "cluster-resources/namespaces-errors.json", marshalErrors(namespaceErrors))
	} else if c.Namespace != "" {
		namespace, namespaceErrors := getNamespace(ctx, client, c.Namespace)
		output.SaveResult(c.BundlePath, "cluster-resources/namespaces.json", bytes.NewBuffer(namespace))
		output.SaveResult(c.BundlePath, "cluster-resources/namespaces-errors.json", marshalErrors(namespaceErrors))
		namespaceNames = append(namespaceNames, c.Namespace)
	} else {
		namespaces, namespaceList, namespaceErrors := getAllNamespaces(ctx, client)
		output.SaveResult(c.BundlePath, "cluster-resources/namespaces.json", bytes.NewBuffer(namespaces))
		output.SaveResult(c.BundlePath, "cluster-resources/namespaces-errors.json", marshalErrors(namespaceErrors))
		if namespaceList != nil {
			for _, namespace := range namespaceList.Items {
				namespaceNames = append(namespaceNames, namespace.Name)
			}
		}
	}

	// pods
	pods, podErrors := pods(ctx, client, namespaceNames)
	for k, v := range pods {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/pods", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/pods-errors.json", marshalErrors(podErrors))

	// services
	services, servicesErrors := services(ctx, client, namespaceNames)
	for k, v := range services {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/services", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/services-errors.json", marshalErrors(servicesErrors))

	// deployments
	deployments, deploymentsErrors := deployments(ctx, client, namespaceNames)
	for k, v := range deployments {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/deployments", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/deployments-errors.json", marshalErrors(deploymentsErrors))

	// statefulsets
	statefulsets, statefulsetsErrors := statefulsets(ctx, client, namespaceNames)
	for k, v := range statefulsets {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/statefulsets", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/statefulsets-errors.json", marshalErrors(statefulsetsErrors))

	// jobs
	jobs, jobsErrors := jobs(ctx, client, namespaceNames)
	for k, v := range jobs {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/jobs", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/jobs-errors.json", marshalErrors(jobsErrors))

	// cronJobs
	cronJobs, cronJobsErrors := cronJobs(ctx, client, namespaceNames)
	for k, v := range cronJobs {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/cronjobs", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/cronjobs-errors.json", marshalErrors(cronJobsErrors))

	// ingress
	ingress, ingressErrors := ingress(ctx, client, namespaceNames)
	for k, v := range ingress {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/ingress", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/ingress-errors.json", marshalErrors(ingressErrors))

	// storage classes
	storageClasses, storageErrors := storageClasses(ctx, client)
	output.SaveResult(c.BundlePath, "cluster-resources/storage-classes.json", bytes.NewBuffer(storageClasses))
	output.SaveResult(c.BundlePath, "cluster-resources/storage-errors.json", marshalErrors(storageErrors))

	// crds
	crdClient, err := apiextensionsv1beta1clientset.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	}
	customResourceDefinitions, crdErrors := crds(ctx, crdClient)
	output.SaveResult(c.BundlePath, "cluster-resources/custom-resource-definitions.json", bytes.NewBuffer(customResourceDefinitions))
	output.SaveResult(c.BundlePath, "cluster-resources/custom-resource-definitions-errors.json", marshalErrors(crdErrors))

	// crs
	customResources, crErrors := crs(ctx, crdClient)
	for k, v := range customResources {
		output.SaveResult(c.BundlePath, fmt.Sprintf("custom-resources/%v", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "custom-resources/custom-resources-errors.json", marshalErrors(crErrors))
	if err != nil {
		return nil, err
	}

	// imagepullsecrets
	imagePullSecrets, pullSecretsErrors := imagePullSecrets(ctx, client, namespaceNames)
	for k, v := range imagePullSecrets {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/image-pull-secrets", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/image-pull-secrets-errors.json", marshalErrors(pullSecretsErrors))

	// nodes
	nodes, nodeErrors := nodes(ctx, client)
	output.SaveResult(c.BundlePath, "cluster-resources/nodes.json", bytes.NewBuffer(nodes))
	output.SaveResult(c.BundlePath, "cluster-resources/nodes-errors.json", marshalErrors(nodeErrors))

	groups, resources, groupsResourcesErrors := apiResources(ctx, client)
	output.SaveResult(c.BundlePath, "cluster-resources/groups.json", bytes.NewBuffer(groups))
	output.SaveResult(c.BundlePath, "cluster-resources/resources.json", bytes.NewBuffer(resources))
	output.SaveResult(c.BundlePath, "cluster-resources/groups-resources-errors.json", marshalErrors(groupsResourcesErrors))

	// limit ranges
	limitRanges, limitRangesErrors := limitRanges(ctx, client, namespaceNames)
	for k, v := range limitRanges {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/limitranges", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/limitranges-errors.json", marshalErrors(limitRangesErrors))

	// auth cani
	authCanI, authCanIErrors := authCanI(ctx, client, namespaceNames)
	for k, v := range authCanI {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/auth-cani-list", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/auth-cani-list-errors.json", marshalErrors(authCanIErrors))

	//Events
	events, eventsErrors := events(ctx, client, namespaceNames)
	for k, v := range events {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/events", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/events-errors.json", marshalErrors(eventsErrors))

	//Persistent Volumes
	pvs, pvsErrors := pvs(ctx, client)
	output.SaveResult(c.BundlePath, "cluster-resources/pvs.json", bytes.NewBuffer(pvs))
	output.SaveResult(c.BundlePath, "cluster-resources/pvs-errors.json", marshalErrors(pvsErrors))

	//Persistent Volume Claims
	pvcs, pvcsErrors := pvcs(ctx, client, namespaceNames)
	for k, v := range pvcs {
		output.SaveResult(c.BundlePath, path.Join("cluster-resources/pvcs", k), bytes.NewBuffer(v))
	}
	output.SaveResult(c.BundlePath, "cluster-resources/pvcs-errors.json", marshalErrors(pvcsErrors))

	return output, nil
}

func getAllNamespaces(ctx context.Context, client *kubernetes.Clientset) ([]byte, *corev1.NamespaceList, []string) {
	namespaces, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, []string{err.Error()}
	}

	b, err := json.MarshalIndent(namespaces.Items, "", "  ")
	if err != nil {
		return nil, nil, []string{err.Error()}
	}

	return b, namespaces, nil
}

func getNamespaces(ctx context.Context, client *kubernetes.Clientset, namespaces []string) ([]byte, []string) {
	namespacesArr := []*corev1.Namespace{}
	errorsArr := []string{}

	for _, namespace := range namespaces {
		ns, err := client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if err != nil {
			errorsArr = append(errorsArr, err.Error())
			continue
		}
		namespacesArr = append(namespacesArr, ns)
	}

	b, err := json.MarshalIndent(namespacesArr, "", "  ")
	if err != nil {
		errorsArr = append(errorsArr, err.Error())
		return nil, errorsArr
	}

	return b, errorsArr
}

func getNamespace(ctx context.Context, client *kubernetes.Clientset, namespace string) ([]byte, []string) {
	ns, err := client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return nil, []string{err.Error()}
	}

	b, err := json.MarshalIndent(ns, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}

	return b, nil
}

func pods(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	podsByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		b, err := json.MarshalIndent(pods.Items, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		podsByNamespace[namespace+".json"] = b
	}

	return podsByNamespace, errorsByNamespace
}

func services(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	servicesByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		services, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		b, err := json.MarshalIndent(services.Items, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		servicesByNamespace[namespace+".json"] = b
	}

	return servicesByNamespace, errorsByNamespace
}

func deployments(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	deploymentsByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		deployments, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		b, err := json.MarshalIndent(deployments.Items, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		deploymentsByNamespace[namespace+".json"] = b
	}

	return deploymentsByNamespace, errorsByNamespace
}

func statefulsets(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	statefulsetsByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		statefulsets, err := client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		b, err := json.MarshalIndent(statefulsets.Items, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		statefulsetsByNamespace[namespace+".json"] = b
	}

	return statefulsetsByNamespace, errorsByNamespace
}

func jobs(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	jobsByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		nsJobs, err := client.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		b, err := json.MarshalIndent(nsJobs.Items, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		jobsByNamespace[namespace+".json"] = b
	}

	return jobsByNamespace, errorsByNamespace
}

func cronJobs(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	cronJobsByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		nsCronJobs, err := client.BatchV1beta1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		b, err := json.MarshalIndent(nsCronJobs.Items, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		cronJobsByNamespace[namespace+".json"] = b
	}

	return cronJobsByNamespace, errorsByNamespace
}

func ingress(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	ingressByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		ingress, err := client.ExtensionsV1beta1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		b, err := json.MarshalIndent(ingress.Items, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		ingressByNamespace[namespace+".json"] = b
	}

	return ingressByNamespace, errorsByNamespace
}

func storageClasses(ctx context.Context, client *kubernetes.Clientset) ([]byte, []string) {
	storageClasses, err := client.StorageV1beta1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, []string{err.Error()}
	}

	b, err := json.MarshalIndent(storageClasses.Items, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}

	return b, nil
}

func crds(ctx context.Context, client *apiextensionsv1beta1clientset.ApiextensionsV1beta1Client) ([]byte, []string) {
	crds, err := client.CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, []string{err.Error()}
	}

	b, err := json.MarshalIndent(crds.Items, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}

	return b, nil
}

func crs(ctx context.Context, client *apiextensionsv1beta1clientset.ApiextensionsV1beta1Client) (map[string][]byte, map[string]string) {
	customResources := make(map[string][]byte)
	errorList := make(map[string]string)
	customResourceItems := struct {
		metav1.TypeMeta `json:",inline"`
		metav1.ListMeta `json:"metadata,omitempty"`
		Items           []map[string]interface{} `json:"items"`
	}{}
	crds, err := client.CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		errorList["crdList"] = err.Error()
		return customResources, errorList
	}
	// Loop through CRDs to fetch the CRs
	for _, v := range crds.Items {
		data := client.RESTClient().Get().AbsPath("/apis/" + v.Spec.Group + "/" + v.Spec.Version).Do(ctx)
		apiResourceListObj, err := data.Get()
		group := v.Spec.Group
		if err != nil {
			errorList[group] = err.Error()
			continue
		}
		apiResourceList, _ := apiResourceListObj.(*metav1.APIResourceList)
		groupVersion := apiResourceList.GroupVersion
		for _, v := range apiResourceList.APIResources {
			customResourceName := v.Name
			if customResourceName != "" && !strings.ContainsAny(customResourceName, "/") {
				fileName := customResourceName + "." + group + ".json"
				customResourcesResponse, err := client.RESTClient().Get().AbsPath("/apis/" + groupVersion).Namespace("").Resource(customResourceName).DoRaw(ctx)
				if err != nil {
					errorList[fileName] = err.Error()
					continue
				}
				_ = json.Unmarshal(customResourcesResponse, &customResourceItems)
				if len(customResourceItems.Items) != 0 {
					customResources[fileName] = customResourcesResponse
				}
			}
		}
	}
	//TODO: Improve formatting of the custom resources output
	return customResources, errorList
}

func imagePullSecrets(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	imagePullSecrets := make(map[string][]byte)
	errors := make(map[string]string)

	// better than vendoring in.... kubernetes
	type DockerConfigEntry struct {
		Auth string `json:"auth"`
	}
	type DockerConfigJSON struct {
		Auths map[string]DockerConfigEntry `json:"auths"`
	}

	for _, namespace := range namespaces {
		secrets, err := client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errors[namespace] = err.Error()
			continue
		}

		for _, secret := range secrets.Items {
			if secret.Type != corev1.SecretTypeDockerConfigJson {
				continue
			}

			dockerConfigJSON := DockerConfigJSON{}
			if err := json.Unmarshal(secret.Data[corev1.DockerConfigJsonKey], &dockerConfigJSON); err != nil {
				errors[fmt.Sprintf("%s/%s", namespace, secret.Name)] = err.Error()
				continue
			}

			for registry, registryAuth := range dockerConfigJSON.Auths {
				decoded, err := base64.StdEncoding.DecodeString(registryAuth.Auth)
				if err != nil {
					errors[fmt.Sprintf("%s/%s/%s", namespace, secret.Name, registry)] = err.Error()
					continue
				}

				registryAndUsername := make(map[string]string)
				registryAndUsername[registry] = strings.Split(string(decoded), ":")[0]
				b, err := json.Marshal(registryAndUsername)
				if err != nil {
					errors[fmt.Sprintf("%s/%s/%s", namespace, secret.Name, registry)] = err.Error()
					continue
				}
				imagePullSecrets[fmt.Sprintf("%s/%s.json", namespace, secret.Name)] = b
			}
		}
	}

	return imagePullSecrets, errors
}

func limitRanges(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	limitRangesByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		limitRanges, err := client.CoreV1().LimitRanges(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		b, err := json.MarshalIndent(limitRanges.Items, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		limitRangesByNamespace[namespace+".json"] = b
	}

	return limitRangesByNamespace, errorsByNamespace
}

func nodes(ctx context.Context, client *kubernetes.Clientset) ([]byte, []string) {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, []string{err.Error()}
	}

	b, err := json.MarshalIndent(nodes.Items, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}

	return b, nil
}

// get the list of API resources, similar to 'kubectl api-resources'
func apiResources(ctx context.Context, client *kubernetes.Clientset) ([]byte, []byte, []string) {
	var errorArray []string
	groups, resources, err := client.Discovery().ServerGroupsAndResources()
	if err != nil {
		errorArray = append(errorArray, err.Error())
	}

	groupBytes, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		errorArray = append(errorArray, err.Error())
	}

	resourcesBytes, err := json.MarshalIndent(resources, "", "  ")
	if err != nil {
		errorArray = append(errorArray, err.Error())
	}

	return groupBytes, resourcesBytes, errorArray
}

func authCanI(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubectl/cmd/auth/cani.go

	authListByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		sar := &authorizationv1.SelfSubjectRulesReview{
			Spec: authorizationv1.SelfSubjectRulesReviewSpec{
				Namespace: namespace,
			},
		}
		response, err := client.AuthorizationV1().SelfSubjectRulesReviews().Create(ctx, sar, metav1.CreateOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		rules := convertToPolicyRule(response.Status)
		b, err := json.MarshalIndent(rules, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		authListByNamespace[namespace+".json"] = b
	}

	return authListByNamespace, errorsByNamespace
}

func events(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	eventsByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		events, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		b, err := json.MarshalIndent(events.Items, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		eventsByNamespace[namespace+".json"] = b
	}

	return eventsByNamespace, errorsByNamespace
}

// not exprted from: https://github.com/kubernetes/kubernetes/blob/master/pkg/kubectl/cmd/auth/cani.go#L339
func convertToPolicyRule(status authorizationv1.SubjectRulesReviewStatus) []rbacv1.PolicyRule {
	ret := []rbacv1.PolicyRule{}
	for _, resource := range status.ResourceRules {
		ret = append(ret, rbacv1.PolicyRule{
			Verbs:         resource.Verbs,
			APIGroups:     resource.APIGroups,
			Resources:     resource.Resources,
			ResourceNames: resource.ResourceNames,
		})
	}

	for _, nonResource := range status.NonResourceRules {
		ret = append(ret, rbacv1.PolicyRule{
			Verbs:           nonResource.Verbs,
			NonResourceURLs: nonResource.NonResourceURLs,
		})
	}

	return ret
}

func pvs(ctx context.Context, client *kubernetes.Clientset) ([]byte, []string) {
	pv, err := client.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, []string{err.Error()}
	}

	b, err := json.MarshalIndent(pv.Items, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}
	return b, nil
}

func pvcs(ctx context.Context, client *kubernetes.Clientset, namespaces []string) (map[string][]byte, map[string]string) {
	pvcsByNamespace := make(map[string][]byte)
	errorsByNamespace := make(map[string]string)

	for _, namespace := range namespaces {
		pvcs, err := client.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		b, err := json.MarshalIndent(pvcs.Items, "", "  ")
		if err != nil {
			errorsByNamespace[namespace] = err.Error()
			continue
		}

		pvcsByNamespace[namespace+".json"] = b
	}

	return pvcsByNamespace, errorsByNamespace
}
