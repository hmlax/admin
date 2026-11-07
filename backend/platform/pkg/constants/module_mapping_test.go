package constants

import (
	operatorV1 "go-admin/platform/pb/operator"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func TestRegisteredServicesHaveAdministrationCategory(t *testing.T) {
	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if file.Package() != "operator.v1" {
			return true
		}
		for i := 0; i < file.Services().Len(); i++ {
			name := string(file.Services().Get(i).Name())
			if name == "GatewayService" {
				continue
			}
			module, ok := ServiceTagToBusinessModule[name]
			if !ok || module == operatorV1.Module_MODULE_UNSPECIFIED {
				t.Errorf("service %s has no administration category", name)
			}
		}
		return true
	})
}
