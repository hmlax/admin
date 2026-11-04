package service

import (
	"google.golang.org/protobuf/proto"

	operatorV1 "go-admin/platform/pb/operator"
	"go-admin/platform/pkg/constants"
)

func filterPlanMenuTree(items []*operatorV1.Menu, allowed map[operatorV1.Module_Module]bool) []*operatorV1.Menu {
	filtered := make([]*operatorV1.Menu, 0, len(items))
	for _, item := range items {
		if item == nil || item.GetType() == operatorV1.Menu_BUTTON {
			continue
		}
		module := item.GetModule()
		if module == operatorV1.Module_MODULE_UNSPECIFIED {
			module = constants.ComponentToModule(item.GetComponent())
		}
		if module != operatorV1.Module_MODULE_UNSPECIFIED && !allowed[module] {
			continue
		}
		children := filterPlanMenuTree(item.Children, allowed)
		catalog := item.GetType() == operatorV1.Menu_CATALOG || item.GetComponent() == "BasicLayout"
		if catalog && len(children) == 0 {
			continue
		}
		// An unclassified business page must not bypass the module whitelist.
		if !catalog && module == operatorV1.Module_MODULE_UNSPECIFIED {
			continue
		}
		result := proto.Clone(item).(*operatorV1.Menu)
		result.Children = children
		if len(children) != len(item.Children) {
			// The old default child may have been removed. The React router will
			// select a visible child instead of redirecting to a denied page.
			result.Redirect = nil
		}
		filtered = append(filtered, result)
	}
	return filtered
}
