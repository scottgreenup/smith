package types

import (
	"github.com/atlassian/smith/pkg/cleanup"
	"github.com/atlassian/smith/pkg/util"
	sc_v1b1 "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"github.com/pkg/errors"
	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	MainKnownTypes = map[schema.GroupKind]cleanup.SpecCleanup{
		{Group: apps_v1.GroupName, Kind: "Deployment"}: deploymentCleanup,
		{Group: core_v1.GroupName, Kind: "Service"}:    serviceCleanup,
		{Group: core_v1.GroupName, Kind: "Secret"}:     secretCleanup,
	}

	ServiceCatalogKnownTypes = map[schema.GroupKind]cleanup.SpecCleanup{
		{Group: sc_v1b1.GroupName, Kind: "ServiceBinding"}:  scServiceBindingCleanup,
		{Group: sc_v1b1.GroupName, Kind: "ServiceInstance"}: scServiceInstanceCleanup,
	}

	appsV1Scheme = runtime.NewScheme()
	scV1B1Scheme = runtime.NewScheme()
	coreV1Scheme = runtime.NewScheme()
)

func init() {
	err := apps_v1.SchemeBuilder.AddToScheme(appsV1Scheme)
	if err != nil {
		panic(err)
	}
	err = sc_v1b1.SchemeBuilder.AddToScheme(scV1B1Scheme)
	if err != nil {
		panic(err)
	}
	err = core_v1.SchemeBuilder.AddToScheme(coreV1Scheme)
	if err != nil {
		panic(err)
	}
}

func deploymentCleanup(spec, actual *unstructured.Unstructured) (runtime.Object, error) {
	var deploymentSpec apps_v1.Deployment
	if err := util.ConvertType(appsV1Scheme, spec, &deploymentSpec); err != nil {
		return nil, err
	}

	deploymentSpec.Spec.Template.Spec.DeprecatedServiceAccount = deploymentSpec.Spec.Template.Spec.ServiceAccountName

	return util.RuntimeToUnstructured(&deploymentSpec)
}

func serviceCleanup(spec, actual *unstructured.Unstructured) (runtime.Object, error) {
	var serviceSpec core_v1.Service
	if err := util.ConvertType(coreV1Scheme, spec, &serviceSpec); err != nil {
		return nil, err
	}
	var serviceActual core_v1.Service
	if err := util.ConvertType(coreV1Scheme, actual, &serviceActual); err != nil {
		return nil, err
	}

	serviceSpec.Spec.ClusterIP = serviceActual.Spec.ClusterIP
	serviceSpec.Status = serviceActual.Status

	if len(serviceActual.Spec.Ports) == len(serviceSpec.Spec.Ports) {
		for i, port := range serviceSpec.Spec.Ports {
			if port.NodePort == 0 {
				actualPort := serviceActual.Spec.Ports[i]
				port.NodePort = actualPort.NodePort
				if port == actualPort { // NodePort field is the only difference, other fields are the same
					// Copy port from actual if port is not specified in spec
					serviceSpec.Spec.Ports[i].NodePort = actualPort.NodePort
				}
			}
		}
	}

	return &serviceSpec, nil
}

func secretCleanup(spec, actual *unstructured.Unstructured) (runtime.Object, error) {
	var secretSpec core_v1.Secret
	if err := util.ConvertType(coreV1Scheme, spec, &secretSpec); err != nil {
		return nil, err
	}

	// StringData overwrites Data
	if len(secretSpec.StringData) > 0 {
		if secretSpec.Data == nil {
			secretSpec.Data = make(map[string][]byte, len(secretSpec.StringData))
		}
		for k, v := range secretSpec.StringData {
			secretSpec.Data[k] = []byte(v)
		}
		secretSpec.StringData = nil
	}

	return &secretSpec, nil
}

func scServiceBindingCleanup(spec, actual *unstructured.Unstructured) (runtime.Object, error) {
	var sbSpec sc_v1b1.ServiceBinding
	if err := util.ConvertType(scV1B1Scheme, spec, &sbSpec); err != nil {
		return nil, err
	}
	var sbActual sc_v1b1.ServiceBinding
	if err := util.ConvertType(scV1B1Scheme, actual, &sbActual); err != nil {
		return nil, err
	}

	sbSpec.Spec.ExternalID = sbActual.Spec.ExternalID
	if sbActual.Spec.UserInfo != nil {
		sbSpec.Spec.UserInfo = sbActual.Spec.UserInfo.DeepCopy()
	}
	sbSpec.Status = sbActual.Status

	return &sbSpec, nil
}

func scServiceInstanceCleanup(spec, actual *unstructured.Unstructured) (runtime.Object, error) {
	var instanceSpec sc_v1b1.ServiceInstance
	if err := util.ConvertType(scV1B1Scheme, spec, &instanceSpec); err != nil {
		return nil, err
	}
	var instanceActual sc_v1b1.ServiceInstance
	if err := util.ConvertType(scV1B1Scheme, actual, &instanceActual); err != nil {
		return nil, err
	}

	if instanceSpec.Spec.ClusterServiceClassName != "" &&
		instanceSpec.Spec.ClusterServiceClassName != instanceActual.Spec.ClusterServiceClassName {
		return nil, errors.New("clusterServiceClassName has changed when it should be immutable")
	}

	if instanceSpec.Spec.ClusterServicePlanName != "" &&
		instanceSpec.Spec.ClusterServicePlanName != instanceActual.Spec.ClusterServicePlanName {
		return nil, errors.New("clusterServicePlanName has changed when it should be immutable")
	}

	if instanceActual.Spec.ClusterServiceClassExternalName == instanceSpec.Spec.ClusterServiceClassExternalName {
		instanceSpec.Spec.ClusterServiceClassRef = instanceActual.Spec.ClusterServiceClassRef
		instanceSpec.Spec.ClusterServiceClassName = instanceActual.Spec.ClusterServiceClassName
	}

	if instanceActual.Spec.ClusterServicePlanExternalName == instanceSpec.Spec.ClusterServicePlanExternalName {
		instanceSpec.Spec.ClusterServicePlanRef = instanceActual.Spec.ClusterServicePlanRef
		instanceSpec.Spec.ClusterServicePlanName = instanceActual.Spec.ClusterServicePlanName
	}

	instanceSpec.ObjectMeta.Finalizers = instanceActual.ObjectMeta.Finalizers

	instanceSpec.Spec.ExternalID = instanceActual.Spec.ExternalID
	if instanceActual.Spec.UserInfo != nil {
		instanceSpec.Spec.UserInfo = instanceActual.Spec.UserInfo
	}

	if instanceSpec.Spec.UpdateRequests == 0 {
		instanceSpec.Spec.UpdateRequests = instanceActual.Spec.UpdateRequests
	}

	return &instanceSpec, nil
}
