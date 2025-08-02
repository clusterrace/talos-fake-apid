package main

import (
	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"google.golang.org/protobuf/proto"
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

func (o ServiceSpec) MarshalProto() ([]byte, error) {
	protoSpec := &v1alpha1.Spec{}
	return proto.Marshal(protoSpec)
}

// NewService initializes a Service resource.
func NewService(id resource.ID) *Service {
	return typed.NewResource[ServiceSpec, ServiceExtension](
		resource.NewMetadata(NamespaceName, ServiceType, id, resource.VersionUndefined),
		ServiceSpec{},
	)
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

func init() {
	err := protobuf.RegisterResource(ServiceType, &Service{})
	if err != nil {
		panic(err)
	}
}
