package service

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	ldap "github.com/go-ldap/ldap/v3"
	"gorm.io/gorm"
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
)

var ErrLDAPUnavailable = errors.New("ldap unavailable")
var ErrLDAPCredentials = errors.New("ldap credentials invalid")

type LDAPIdentity struct {
	Username, DisplayName, Email string
	UserDN                       string
	Groups                       []string
}

type LDAPService struct{ store repository.UserStore }

func NewLDAPService(store repository.UserStore) *LDAPService { return &LDAPService{store: store} }

func safeLDAPConfig(item model.LDAPConfig) model.LDAPConfigView {
	item.ApplyDefaults()
	return model.LDAPConfigView{LDAPConfig: item, HasBindPassword: strings.TrimSpace(item.BindPassword) != ""}
}
func (s *LDAPService) View(ctx context.Context) (model.LDAPConfigView, error) {
	item, err := s.store.GetLDAPConfig(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return safeLDAPConfig(model.DefaultLDAPConfig()), nil
	}
	if err != nil {
		return model.LDAPConfigView{}, err
	}
	return safeLDAPConfig(item), nil
}

func normalizeLDAPConfig(item *model.LDAPConfig) {
	item.URL = strings.TrimSpace(item.URL)
	item.BindDN = strings.TrimSpace(item.BindDN)
	item.BaseDN = strings.TrimSpace(item.BaseDN)
	item.GroupSearchBaseDN = strings.TrimSpace(item.GroupSearchBaseDN)
	item.UserFilter = strings.TrimSpace(item.UserFilter)
	item.GroupFilter = strings.TrimSpace(item.GroupFilter)
	item.GroupAttribute = strings.TrimSpace(item.GroupAttribute)
	if item.UserFilter == "" {
		item.UserFilter = "(|(uid={{username}})(sAMAccountName={{username}}))"
	}
	if item.UsernameAttribute == "" {
		item.UsernameAttribute = "uid"
	}
	if item.DisplayNameAttribute == "" {
		item.DisplayNameAttribute = "displayName"
	}
	if item.EmailAttribute == "" {
		item.EmailAttribute = "mail"
	}
	if item.DefaultRole == "" {
		item.DefaultRole = "user"
	}
	if item.GroupFilter == "" {
		item.GroupFilter = "(&(objectClass=groupOfNames)(member={{userDN}}))"
	}
	if item.GroupAttribute == "" {
		item.GroupAttribute = "cn"
	}
	if item.GroupSearchBaseDN == "" {
		item.GroupSearchBaseDN = item.BaseDN
	}
}

func ldapURL(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("%w: LDAP 地址不能为空", ErrLDAPUnavailable)
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "ldap" && parsed.Scheme != "ldaps") {
		return "", fmt.Errorf("%w: LDAP 地址格式应为 ldap:// 或 ldaps://", ErrLDAPUnavailable)
	}
	return raw, nil
}

const ldapOperationTimeout = 15 * time.Second

// Bound the entire exchange, including TLS negotiation and LDAP responses. The
// library's default only bounds TCP dialing; a silent directory could otherwise
// retain a login request and its credentials indefinitely.
func dialLDAP(ctx context.Context, cfg model.LDAPConfig) (*ldap.Conn, func(), error) {
	raw, err := ldapURL(cfg.URL)
	if err != nil {
		return nil, nil, err
	}
	u, _ := url.Parse(raw)
	port := u.Port()
	if port == "" {
		port = "389"
		if u.Scheme == "ldaps" {
			port = "636"
		}
	}
	deadline := time.Now().Add(ldapOperationTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	dialer := net.Dialer{Deadline: deadline}
	socket, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(u.Hostname(), port))
	if err != nil {
		return nil, nil, fmt.Errorf("%w: 无法连接 LDAP 服务器", ErrLDAPUnavailable)
	}
	_ = socket.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { _ = socket.Close() })
	tlsConfig := &tls.Config{ServerName: u.Hostname(), InsecureSkipVerify: cfg.SkipTLSVerify, MinVersion: tls.VersionTLS12}
	var transport net.Conn = socket
	if u.Scheme == "ldaps" {
		tlsConn := tls.Client(socket, tlsConfig)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			stop()
			_ = socket.Close()
			return nil, nil, fmt.Errorf("%w: LDAP TLS 握手失败", ErrLDAPUnavailable)
		}
		transport = tlsConn
	}
	conn := ldap.NewConn(transport, u.Scheme == "ldaps")
	conn.SetTimeout(ldapOperationTimeout)
	conn.Start()
	closeConn := func() { stop(); _ = conn.Close() }
	if cfg.UseTLS && u.Scheme == "ldap" {
		if err := conn.StartTLS(tlsConfig); err != nil {
			closeConn()
			return nil, nil, fmt.Errorf("%w: LDAP TLS 握手失败", ErrLDAPUnavailable)
		}
	}
	return conn, closeConn, nil
}
func bindService(conn *ldap.Conn, cfg model.LDAPConfig) error {
	if cfg.BindDN == "" {
		return nil
	}
	if err := conn.Bind(cfg.BindDN, cfg.BindPassword); err != nil {
		return fmt.Errorf("%w: LDAP 服务账号绑定失败", ErrLDAPCredentials)
	}
	return nil
}
func (s *LDAPService) Test(ctx context.Context, cfg model.LDAPConfig) error {
	normalizeLDAPConfig(&cfg)
	conn, closeConn, err := dialLDAP(ctx, cfg)
	if err != nil {
		return err
	}
	defer closeConn()
	return bindService(conn, cfg)
}

func (s *LDAPService) Authenticate(ctx context.Context, username, password string) (LDAPIdentity, error) {
	cfg, err := s.store.GetLDAPConfig(ctx)
	if err != nil || !cfg.Enabled {
		return LDAPIdentity{}, ErrLDAPUnavailable
	}
	normalizeLDAPConfig(&cfg)
	conn, closeConn, err := dialLDAP(ctx, cfg)
	if err != nil {
		return LDAPIdentity{}, err
	}
	defer closeConn()
	if err := bindService(conn, cfg); err != nil {
		return LDAPIdentity{}, err
	}
	filter := strings.ReplaceAll(cfg.UserFilter, "{{username}}", ldap.EscapeFilter(username))
	attrs := []string{cfg.UsernameAttribute, cfg.DisplayNameAttribute, cfg.EmailAttribute}
	result, err := conn.Search(ldap.NewSearchRequest(cfg.BaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 2, 10, false, filter, attrs, nil))
	if err != nil || len(result.Entries) == 0 {
		return LDAPIdentity{}, ErrLDAPCredentials
	}
	if len(result.Entries) > 1 {
		return LDAPIdentity{}, fmt.Errorf("%w: LDAP 用户匹配到多个账号", ErrLDAPCredentials)
	}
	entry := result.Entries[0]
	if err := conn.Bind(entry.DN, password); err != nil {
		return LDAPIdentity{}, ErrLDAPCredentials
	}
	display := entry.GetAttributeValue(cfg.DisplayNameAttribute)
	if display == "" {
		display = username
	}
	// The user bind proves the supplied credentials. Rebind with the configured
	// service account before reading groups because LDAP ACLs commonly allow
	// authentication but do not allow users to enumerate group memberships.
	if err := bindService(conn, cfg); err != nil {
		return LDAPIdentity{}, err
	}
	groups, groupErr := searchUserGroups(conn, cfg, entry.DN, username)
	if groupErr != nil {
		return LDAPIdentity{}, groupErr
	}
	return LDAPIdentity{Username: entry.GetAttributeValue(cfg.UsernameAttribute), DisplayName: display, Email: entry.GetAttributeValue(cfg.EmailAttribute), UserDN: entry.DN, Groups: groups}, nil
}

func searchUserGroups(conn *ldap.Conn, cfg model.LDAPConfig, userDN, username string) ([]string, error) {
	base := cfg.GroupSearchBaseDN
	if base == "" {
		base = cfg.BaseDN
	}
	filter := strings.ReplaceAll(cfg.GroupFilter, "{{userDN}}", ldap.EscapeFilter(userDN))
	filter = strings.ReplaceAll(filter, "{{username}}", ldap.EscapeFilter(username))
	// Group membership is plural. The size limit of two is only appropriate for
	// the user lookup, where it detects an ambiguous identity.
	result, err := conn.Search(ldap.NewSearchRequest(base, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 10, false, filter, []string{cfg.GroupAttribute, "dn"}, nil))
	if err != nil {
		return nil, fmt.Errorf("%w: LDAP 用户组查询失败", ErrLDAPUnavailable)
	}
	groups := make([]string, 0, len(result.Entries))
	for _, entry := range result.Entries {
		value := strings.TrimSpace(entry.GetAttributeValue(cfg.GroupAttribute))
		if value == "" {
			value = entry.DN
		}
		groups = append(groups, value)
	}
	return groups, nil
}

// LDAPRoleCodes resolves configured group mappings. The default role is used when
// no mapping matches, keeping existing deployments safe when group mapping is empty.
func LDAPRoleCodes(cfg model.LDAPConfig, groups []string) []string {
	cfg.ApplyDefaults()
	matched := make([]string, 0)
	seen := map[string]bool{}
	for _, group := range groups {
		for _, mapping := range cfg.GroupRoleMappings {
			if strings.EqualFold(strings.TrimSpace(mapping.Group), strings.TrimSpace(group)) && strings.TrimSpace(mapping.RoleCode) != "" && !seen[mapping.RoleCode] {
				seen[mapping.RoleCode] = true
				matched = append(matched, mapping.RoleCode)
			}
		}
	}
	if len(matched) == 0 && strings.TrimSpace(cfg.DefaultRole) != "" {
		return []string{cfg.DefaultRole}
	}
	return matched
}
