package auth

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"
	"gorm.io/gorm"

	"devops/internal/models"
	"devops/internal/modules/auth/repository"
	"devops/pkg/logger"
)

var log = logger.L().WithField("module", "ldap")

var (
	ErrLDAPDisabled    = errors.New("LDAP 未启用")
	ErrLDAPConnect     = errors.New("LDAP 连接失败")
	ErrLDAPBind        = errors.New("LDAP 认证失败")
	ErrLDAPUserSearch  = errors.New("LDAP 用户查询失败")
	ErrLDAPUserNotFound = errors.New("LDAP 用户不存在")
)

type LDAPConfig struct {
	Enabled       bool   `json:"enabled"`
	Server        string `json:"server"`
	Port          int    `json:"port"`
	UseTLS        bool   `json:"use_tls"`
	SkipVerify    bool   `json:"skip_verify"`
	BindDN        string `json:"bind_dn"`
	BindPassword  string `json:"bind_password"`
	BaseDN        string `json:"base_dn"`
	UserFilter    string `json:"user_filter"`
	AttrUsername  string `json:"attr_username"`
	AttrEmail     string `json:"attr_email"`
	AttrPhone     string `json:"attr_phone"`
	AttrRealName  string `json:"attr_real_name"`
	GroupBaseDN   string `json:"group_base_dn"`
	GroupFilter   string `json:"group_filter"`
	GroupAttrName string `json:"group_attr_name"`
	GroupAttrMember string `json:"group_attr_member"`
}

func DefaultLDAPConfig() *LDAPConfig {
	return &LDAPConfig{
		Port:           389,
		UserFilter:     "(uid=%s)",
		AttrUsername:   "uid",
		AttrEmail:      "mail",
		AttrPhone:      "telephoneNumber",
		AttrRealName:   "cn",
		GroupFilter:    "(objectClass=groupOfNames)",
		GroupAttrName:  "cn",
		GroupAttrMember: "member",
	}
}

type LDAPGroupMapping struct {
	ID       uint   `gorm:"primarykey" json:"id"`
	GroupDN  string `gorm:"size:500;not null;uniqueIndex" json:"group_dn"`
	GroupName string `gorm:"size:200;not null" json:"group_name"`
	RoleID   uint   `gorm:"not null;index" json:"role_id"`
	RoleName string `gorm:"-" json:"role_name,omitempty"`
}

func (LDAPGroupMapping) TableName() string { return "ldap_group_mappings" }

type LDAPUserResult struct {
	Username string
	Email    string
	Phone    string
	RealName string
	DN       string
	Groups   []string
}

type LDAPService struct {
	configRepo *repository.SystemConfigRepository
	userRepo   *repository.UserRepository
	roleRepo   *repository.RoleRepository
	urRepo     *repository.UserRoleRepository
	db         *gorm.DB
}

func NewLDAPService(db *gorm.DB, configRepo *repository.SystemConfigRepository, userRepo *repository.UserRepository, roleRepo *repository.RoleRepository, urRepo *repository.UserRoleRepository) *LDAPService {
	return &LDAPService{db: db, configRepo: configRepo, userRepo: userRepo, roleRepo: roleRepo, urRepo: urRepo}
}

const ldapConfigKey = "ldap:config"

func (s *LDAPService) GetConfig(ctx context.Context) (*LDAPConfig, error) {
	val, err := s.configRepo.Get(ctx, ldapConfigKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DefaultLDAPConfig(), nil
		}
		return nil, err
	}
	cfg := DefaultLDAPConfig()
	if err := json.Unmarshal([]byte(val), cfg); err != nil {
		return DefaultLDAPConfig(), nil
	}
	return cfg, nil
}

func (s *LDAPService) SaveConfig(ctx context.Context, cfg *LDAPConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return s.configRepo.Set(ctx, ldapConfigKey, string(data), "LDAP 认证配置")
}

func (s *LDAPService) TestConnection(ctx context.Context, cfg *LDAPConfig) error {
	conn, err := s.dial(cfg)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrLDAPConnect, err)
	}
	defer conn.Close()

	if err := conn.Bind(cfg.BindDN, cfg.BindPassword); err != nil {
		return fmt.Errorf("%w: %v", ErrLDAPBind, err)
	}
	return nil
}

// Authenticate 通过 LDAP 认证用户，返回用户信息和所属组
func (s *LDAPService) Authenticate(ctx context.Context, username, password string) (*LDAPUserResult, error) {
	cfg, err := s.GetConfig(ctx)
	if err != nil || !cfg.Enabled {
		return nil, ErrLDAPDisabled
	}

	conn, err := s.dial(cfg)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLDAPConnect, err)
	}
	defer conn.Close()

	if err := conn.Bind(cfg.BindDN, cfg.BindPassword); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLDAPBind, err)
	}

	filter := fmt.Sprintf(cfg.UserFilter, ldap.EscapeFilter(username))
	searchReq := ldap.NewSearchRequest(
		cfg.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, 0, false,
		filter,
		[]string{"dn", cfg.AttrUsername, cfg.AttrEmail, cfg.AttrPhone, cfg.AttrRealName},
		nil,
	)
	sr, err := conn.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLDAPUserSearch, err)
	}
	if len(sr.Entries) == 0 {
		return nil, ErrLDAPUserNotFound
	}

	entry := sr.Entries[0]
	userDN := entry.DN

	if err := conn.Bind(userDN, password); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLDAPBind, err)
	}

	// rebind as admin to search groups
	if err := conn.Bind(cfg.BindDN, cfg.BindPassword); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLDAPBind, err)
	}

	result := &LDAPUserResult{
		Username: entry.GetAttributeValue(cfg.AttrUsername),
		Email:    entry.GetAttributeValue(cfg.AttrEmail),
		Phone:    entry.GetAttributeValue(cfg.AttrPhone),
		RealName: entry.GetAttributeValue(cfg.AttrRealName),
		DN:       userDN,
	}
	if result.Username == "" {
		result.Username = username
	}

	result.Groups = s.searchGroups(conn, cfg, userDN)
	return result, nil
}

func (s *LDAPService) searchGroups(conn *ldap.Conn, cfg *LDAPConfig, userDN string) []string {
	if cfg.GroupBaseDN == "" {
		return nil
	}
	filter := fmt.Sprintf("(&%s(%s=%s))", cfg.GroupFilter, cfg.GroupAttrMember, ldap.EscapeFilter(userDN))
	searchReq := ldap.NewSearchRequest(
		cfg.GroupBaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		[]string{"dn", cfg.GroupAttrName},
		nil,
	)
	sr, err := conn.Search(searchReq)
	if err != nil {
		log.WithError(err).Warn("LDAP 组查询失败")
		return nil
	}
	groups := make([]string, 0, len(sr.Entries))
	for _, e := range sr.Entries {
		groups = append(groups, e.DN)
	}
	return groups
}

// SyncUserFromLDAP 根据 LDAP 认证结果同步/创建本地用户，并映射角色
func (s *LDAPService) SyncUserFromLDAP(ctx context.Context, ldapUser *LDAPUserResult) (*models.User, error) {
	user, err := s.userRepo.GetByUsername(ctx, ldapUser.Username)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if user == nil {
		email := ldapUser.Email
		if email == "" {
			email = ldapUser.Username + "@ldap.local"
		}
		user = &models.User{
			Username: ldapUser.Username,
			Password: "LDAP_AUTH",
			Email:    email,
			Phone:    ldapUser.Phone,
			Role:     "user",
			Status:   "active",
		}
		if err := s.userRepo.Create(ctx, user); err != nil {
			if strings.Contains(err.Error(), "Duplicate") {
				user, _ = s.userRepo.GetByUsername(ctx, ldapUser.Username)
			} else {
				return nil, err
			}
		}
		log.WithField("username", ldapUser.Username).Info("LDAP 用户自动创建")
	}

	s.syncGroupRoles(ctx, user.ID, ldapUser.Groups)
	return user, nil
}

func (s *LDAPService) syncGroupRoles(ctx context.Context, userID uint, ldapGroups []string) {
	if len(ldapGroups) == 0 {
		return
	}
	var mappings []LDAPGroupMapping
	if err := s.db.WithContext(ctx).Find(&mappings).Error; err != nil {
		return
	}
	groupSet := make(map[string]struct{}, len(ldapGroups))
	for _, g := range ldapGroups {
		groupSet[strings.ToLower(g)] = struct{}{}
	}

	roleIDs := make(map[uint]struct{})
	for _, m := range mappings {
		if _, ok := groupSet[strings.ToLower(m.GroupDN)]; ok {
			roleIDs[m.RoleID] = struct{}{}
		}
	}

	if len(roleIDs) == 0 {
		return
	}

	ids := make([]uint, 0, len(roleIDs))
	for id := range roleIDs {
		ids = append(ids, id)
	}

	existingIDs, _ := s.urRepo.GetUserRoleIDs(ctx, userID)
	existingSet := make(map[uint]struct{}, len(existingIDs))
	for _, id := range existingIDs {
		existingSet[id] = struct{}{}
	}
	merged := make(map[uint]struct{})
	for id := range existingSet {
		merged[id] = struct{}{}
	}
	for _, id := range ids {
		merged[id] = struct{}{}
	}

	if len(merged) == len(existingSet) {
		return
	}

	allIDs := make([]uint, 0, len(merged))
	for id := range merged {
		allIDs = append(allIDs, id)
	}
	s.urRepo.SetUserRoles(ctx, userID, allIDs)
	log.WithField("user_id", userID).WithField("role_ids", allIDs).Info("LDAP 组→角色同步")
}

// --- Group mapping CRUD ---

func (s *LDAPService) ListGroupMappings(ctx context.Context) ([]LDAPGroupMapping, error) {
	var list []LDAPGroupMapping
	if err := s.db.WithContext(ctx).Find(&list).Error; err != nil {
		return nil, err
	}
	roles, _, _ := s.roleRepo.List(ctx, 1, 1000)
	roleMap := make(map[uint]string)
	for _, r := range roles {
		roleMap[r.ID] = r.DisplayName
		if roleMap[r.ID] == "" {
			roleMap[r.ID] = r.Name
		}
	}
	for i := range list {
		list[i].RoleName = roleMap[list[i].RoleID]
	}
	return list, nil
}

func (s *LDAPService) CreateGroupMapping(ctx context.Context, m *LDAPGroupMapping) error {
	return s.db.WithContext(ctx).Create(m).Error
}

func (s *LDAPService) UpdateGroupMapping(ctx context.Context, m *LDAPGroupMapping) error {
	return s.db.WithContext(ctx).Save(m).Error
}

func (s *LDAPService) DeleteGroupMapping(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&LDAPGroupMapping{}, id).Error
}

func (s *LDAPService) dial(cfg *LDAPConfig) (*ldap.Conn, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Server, cfg.Port)
	if cfg.UseTLS {
		return ldap.DialTLS("tcp", addr, &tls.Config{
			InsecureSkipVerify: cfg.SkipVerify,
		})
	}
	return ldap.DialURL(fmt.Sprintf("ldap://%s", addr))
}
