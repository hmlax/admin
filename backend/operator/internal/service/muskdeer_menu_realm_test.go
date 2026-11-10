package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	pb "go-admin/platform/pb/operator"
	"go-admin/platform/pkg/muskdeer"
)

func TestMuskdeerNavigationRejectsWrongRealmEvenWithStaleMenuGrants(t *testing.T) {
	s := &AdminPortalService{}
	var menus []*pb.Menu
	for _, page := range muskdeer.Pages() {
		menus = append(menus, &pb.Menu{
			Name:   &page.Name,
			Type:   pb.Menu_MENU.Enum(),
			Status: pb.Menu_ON.Enum(),
		})
	}
	for _, platform := range []bool{true, false} {
		got := s.fillRouteItem(menus, platform)
		var names []string
		for _, row := range got {
			names = append(names, row.GetName())
		}
		for _, page := range muskdeer.Pages() {
			if page.Allows(platform) {
				require.Contains(t, names, page.Name)
			} else {
				require.NotContains(t, names, page.Name)
			}
		}
	}
}

func TestMuskdeerRetiredPublicationMenuIgnoresStaleGrants(t *testing.T) {
	for _, platform := range []bool{true, false} {
		require.False(t, muskdeer.MenuAllowed(muskdeer.RetiredSyntheticMenuName, platform))
		require.False(t, muskdeer.MenuAllowed(muskdeer.RetiredOverviewMenuName, platform))
	}
}
