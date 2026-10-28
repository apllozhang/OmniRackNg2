package model

// LDAPGroupRoleMapping maps a directory group value (usually cn or full DN) to an application role.
type LDAPGroupRoleMapping struct {
	Group    string `json:"group"`
	RoleCode string `json:"roleCode"`
}

// LDAPConfig stores the administrator-managed directory integration settings.
// BindPassword is never serialized to JSON; clients only receive HasBindPassword.
type LDAPConfig struct {
	BaseModel
	Enabled              bool                   `gorm:"not null;default:false" json:"enabled"`
	URL                  string                 `gorm:"size:500;not null;default:''" json:"url"`
	UseTLS               bool                   `gorm:"not null;default:false" json:"useTls"`
	SkipTLSVerify        bool                   `gorm:"not null;default:false" json:"skipTlsVerify"`
	BindDN               string                 `gorm:"size:255;not null;default:''" json:"bindDn"`
	BindPassword         string                 `gorm:"size:500;not null;default:''" json:"-"`
	BaseDN               string                 `gorm:"size:255;not null;default:''" json:"baseDn"`
	UserFilter           string                 `gorm:"size:500;not null" json:"userFilter"`
	UsernameAttribute    string                 `gorm:"size:100;not null;default:uid" json:"usernameAttribute"`
	DisplayNameAttribute string                 `gorm:"size:100;not null;default:displayName" json:"displayNameAttribute"`
	EmailAttribute       string                 `gorm:"size:100;not null;default:mail" json:"emailAttribute"`
	DefaultRole          string                 `gorm:"size:100;not null;default:user" json:"defaultRole"`
	AllowLocalFallback   bool                   `gorm:"not null;default:true" json:"allowLocalFallback"`
	GroupSearchBaseDN    string                 `gorm:"size:255;not null;default:''" json:"groupSearchBaseDn"`
	GroupFilter          string                 `gorm:"size:500;not null;default:''" json:"groupFilter"`
	GroupAttribute       string                 `gorm:"size:100;not null;default:cn" json:"groupAttribute"`
	GroupRoleMappings    []LDAPGroupRoleMapping `gorm:"serializer:json;type:jsonb" json:"groupRoleMappings"`
}

type LDAPConfigView struct {
	LDAPConfig
	BindPassword    string `json:"-"`
	HasBindPassword bool   `json:"hasBindPassword"`
}

// DefaultLDAPConfig returns safe defaults for a newly created directory configuration.
func DefaultLDAPConfig() LDAPConfig {
	return LDAPConfig{
		UserFilter:           "(|(uid={{username}})(sAMAccountName={{username}}))",
		UsernameAttribute:    "uid",
		DisplayNameAttribute: "displayName",
		EmailAttribute:       "mail",
		DefaultRole:          "user",
		AllowLocalFallback:   true,
		GroupFilter:          "(&(objectClass=groupOfNames)(member={{userDN}}))",
		GroupAttribute:       "cn",
		GroupRoleMappings:    []LDAPGroupRoleMapping{},
	}
}

// ApplyDefaults fills only omitted string settings and is safe to call on existing data.
func (c *LDAPConfig) ApplyDefaults() {
	defaults := DefaultLDAPConfig()
	if c.UserFilter == "" {
		c.UserFilter = defaults.UserFilter
	}
	if c.UsernameAttribute == "" {
		c.UsernameAttribute = defaults.UsernameAttribute
	}
	if c.DisplayNameAttribute == "" {
		c.DisplayNameAttribute = defaults.DisplayNameAttribute
	}
	if c.EmailAttribute == "" {
		c.EmailAttribute = defaults.EmailAttribute
	}
	if c.DefaultRole == "" {
		c.DefaultRole = defaults.DefaultRole
	}
	if c.GroupFilter == "" {
		c.GroupFilter = defaults.GroupFilter
	}
	if c.GroupAttribute == "" {
		c.GroupAttribute = defaults.GroupAttribute
	}
	if c.GroupRoleMappings == nil {
		c.GroupRoleMappings = []LDAPGroupRoleMapping{}
	}
	if c.GroupSearchBaseDN == "" {
		c.GroupSearchBaseDN = c.BaseDN
	}
}
