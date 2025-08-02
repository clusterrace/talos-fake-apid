package main

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// ServiceType is type of Service resource.
const ServiceType = resource.Type("Services.v1alpha1.talos.dev")

// NamespaceName contains resources linking v1alpha2 components with v1alpha1 Talos runtime.
const NamespaceName resource.Namespace = "runtime"

// Service describes running service state.
type Service = typed.Resource[ServiceSpec, ServiceExtension]

type ServiceSpec struct {
	Running bool `yaml:"running" protobuf:"1"`
	Healthy bool `yaml:"healthy" protobuf:"2"`
	Unknown bool `yaml:"unknown" protobuf:"3"`
}

// DeepCopy generates a deep copy of ServiceSpec.
func (o ServiceSpec) DeepCopy() ServiceSpec {
	var cp ServiceSpec = o
	return cp
}

// ServiceExtension provides auxiliary methods for Service.
type ServiceExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ServiceExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ServiceType,
		Aliases:          []resource.Type{"svc"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Running",
				JSONPath: "{.running}",
			},
			{
				Name:     "Healthy",
				JSONPath: "{.healthy}",
			},
			{
				Name:     "Health Unknown",
				JSONPath: "{.unknown}",
			},
		},
	}
}
