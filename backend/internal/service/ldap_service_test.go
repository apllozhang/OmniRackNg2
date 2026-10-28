package service

import (
	"testing"

	"omnirack-replica/backend/internal/model"
)

func TestLDAPRoleCodesUsesMappedRolesAndDefaultFallback(t *testing.T) {
	cfg := model.DefaultLDAPConfig()
	cfg.DefaultRole = "user"
	cfg.GroupRoleMappings = []model.LDAPGroupRoleMapping{
		{Group: "Cabinet-Admins", RoleCode: "system_admin"},
		{Group: "cabinet-users", RoleCode: "user"},
	}

	got := LDAPRoleCodes(cfg, []string{"cabinet-admins", "CABINET-ADMINS"})
	if len(got) != 1 || got[0] != "system_admin" {
		t.Fatalf("expected one case-insensitive mapped role, got %#v", got)
	}

	got = LDAPRoleCodes(cfg, []string{"unmapped-group"})
	if len(got) != 1 || got[0] != "user" {
		t.Fatalf("expected default role for an unmapped group, got %#v", got)
	}
}

func TestLDAPRoleCodesSupportsMultipleMappedRolesWithoutDuplicates(t *testing.T) {
	cfg := model.DefaultLDAPConfig()
	cfg.DefaultRole = "user"
	cfg.GroupRoleMappings = []model.LDAPGroupRoleMapping{
		{Group: "cabinet-users", RoleCode: "user"},
		{Group: "cabinet-admins", RoleCode: "system_admin"},
	}
	got := LDAPRoleCodes(cfg, []string{"cabinet-users", "cabinet-admins", "CABINET-USERS"})
	if len(got) != 2 || got[0] != "user" || got[1] != "system_admin" {
		t.Fatalf("expected stable unique multi-role mapping, got %#v", got)
	}
}
