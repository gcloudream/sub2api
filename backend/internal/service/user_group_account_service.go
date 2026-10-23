package service

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 用户侧「账号管理」：被分配了专属分组的普通用户可以只读查看该分组下的上游账号。
//
// 对外只暴露账号名称、状态与用量窗口（使用率 + 重置时间）；凭据、代理、错误详情、
// 窗口内消费统计等字段一律不出本层，handler 直接序列化这里的白名单结构。

var (
	ErrUserGroupAccountNotFound = infraerrors.NotFound("GROUP_ACCOUNT_NOT_FOUND", "account not found")
	ErrUserGroupAccountUsage    = infraerrors.ServiceUnavailable("GROUP_ACCOUNT_USAGE_UNAVAILABLE", "usage query failed, please try again later")
)

// 用户可见的账号状态，按优先级从高到低判定。
const (
	UserGroupAccountStatusDisabled          = "disabled"
	UserGroupAccountStatusError             = "error"
	UserGroupAccountStatusExpired           = "expired"
	UserGroupAccountStatusPaused            = "paused"
	UserGroupAccountStatusTempUnschedulable = "temp_unschedulable"
	UserGroupAccountStatusRateLimited       = "rate_limited"
	UserGroupAccountStatusOverloaded        = "overloaded"
	UserGroupAccountStatusActive            = "active"
)

// UserGroupAccountUsageSource 用量来源：passive 读已采样快照（不请求上游），active 主动查询。
const (
	UserGroupAccountUsageSourcePassive = "passive"
	UserGroupAccountUsageSourceActive  = "active"
)

type UserGroupAccountGroup struct {
	ID       int64              `json:"id"`
	Name     string             `json:"name"`
	Platform string             `json:"platform"`
	Accounts []UserGroupAccount `json:"accounts"`
}

type UserGroupAccount struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Platform    string     `json:"platform"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	StatusUntil *time.Time `json:"status_until,omitempty"`
	// PassiveUsage 为 true 时前端首屏可用 passive 来源，避免一打开页面就请求上游。
	PassiveUsage bool `json:"passive_usage"`
}

type UserGroupAccountUsageWindow struct {
	Utilization      float64    `json:"utilization"`
	ResetsAt         *time.Time `json:"resets_at"`
	RemainingSeconds int        `json:"remaining_seconds"`
}

type UserGroupAccountUsage struct {
	Source         string                       `json:"source,omitempty"`
	UpdatedAt      *time.Time                   `json:"updated_at,omitempty"`
	FiveHour       *UserGroupAccountUsageWindow `json:"five_hour,omitempty"`
	SevenDay       *UserGroupAccountUsageWindow `json:"seven_day,omitempty"`
	SevenDaySonnet *UserGroupAccountUsageWindow `json:"seven_day_sonnet,omitempty"`
	SevenDayFable  *UserGroupAccountUsageWindow `json:"seven_day_fable,omitempty"`
}

type userGroupAccountUserReader interface {
	GetByID(ctx context.Context, id int64) (*User, error)
}

type userGroupAccountGroupReader interface {
	GetByIDLite(ctx context.Context, id int64) (*Group, error)
}

type userGroupAccountAccountReader interface {
	GetByID(ctx context.Context, id int64) (*Account, error)
	ListAllWithFilters(ctx context.Context, platform, accountType, status, search string, groupID int64, privacyMode string) ([]Account, error)
}

type userGroupAccountUsageReader interface {
	GetPassiveUsage(ctx context.Context, accountID int64) (*UsageInfo, error)
	GetUsageForAccount(ctx context.Context, account *Account, force ...bool) (*UsageInfo, error)
}

type UserGroupAccountService struct {
	userRepo     userGroupAccountUserReader
	groupRepo    userGroupAccountGroupReader
	accountRepo  userGroupAccountAccountReader
	usageService userGroupAccountUsageReader
	now          func() time.Time
}

func NewUserGroupAccountService(
	userRepo UserRepository,
	groupRepo GroupRepository,
	accountRepo AccountRepository,
	usageService *AccountUsageService,
) *UserGroupAccountService {
	return &UserGroupAccountService{
		userRepo:     userRepo,
		groupRepo:    groupRepo,
		accountRepo:  accountRepo,
		usageService: usageService,
		now:          time.Now,
	}
}

// visibleGroups 返回用户被分配的、仍处于启用状态的专属分组，按名称排序。
func (s *UserGroupAccountService) visibleGroups(ctx context.Context, userID int64) ([]*Group, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	groups := make([]*Group, 0, len(user.AllowedGroups))
	seen := make(map[int64]struct{}, len(user.AllowedGroups))
	for _, groupID := range user.AllowedGroups {
		if _, dup := seen[groupID]; dup || groupID <= 0 {
			continue
		}
		seen[groupID] = struct{}{}
		group, err := s.groupRepo.GetByIDLite(ctx, groupID)
		if err != nil {
			if infraerrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		if group == nil || !group.IsExclusive || !group.IsActive() {
			continue
		}
		groups = append(groups, group)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return strings.ToLower(groups[i].Name) < strings.ToLower(groups[j].Name)
	})
	return groups, nil
}

// ListGroups 返回用户可查看的专属分组及其账号（不含用量，用量按账号单独查询）。
func (s *UserGroupAccountService) ListGroups(ctx context.Context, userID int64) ([]UserGroupAccountGroup, error) {
	groups, err := s.visibleGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	out := make([]UserGroupAccountGroup, 0, len(groups))
	for _, group := range groups {
		accounts, err := s.accountRepo.ListAllWithFilters(ctx, "", "", "", "", group.ID, "")
		if err != nil {
			return nil, err
		}
		views := make([]UserGroupAccount, 0, len(accounts))
		for i := range accounts {
			views = append(views, userGroupAccountView(&accounts[i], now))
		}
		sort.SliceStable(views, func(i, j int) bool {
			return strings.ToLower(views[i].Name) < strings.ToLower(views[j].Name)
		})
		out = append(out, UserGroupAccountGroup{
			ID:       group.ID,
			Name:     group.Name,
			Platform: group.Platform,
			Accounts: views,
		})
	}
	return out, nil
}

// GetUsage 查询单个账号的用量窗口。账号必须属于用户可查看的某个专属分组，否则一律按不存在处理，
// 不泄露账号是否存在。active 来源沿用用量服务自身的缓存与探测节流，从不强制绕过。
func (s *UserGroupAccountService) GetUsage(ctx context.Context, userID, accountID int64, source string) (*UserGroupAccountUsage, error) {
	if accountID <= 0 {
		return nil, ErrUserGroupAccountNotFound
	}
	groups, err := s.visibleGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, ErrUserGroupAccountNotFound
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		if infraerrors.IsNotFound(err) {
			return nil, ErrUserGroupAccountNotFound
		}
		return nil, err
	}
	if account == nil || !accountInGroups(account, groups) {
		return nil, ErrUserGroupAccountNotFound
	}

	var usage *UsageInfo
	if source == UserGroupAccountUsageSourcePassive && supportsAnthropicPassiveUsage(account) {
		usage, err = s.usageService.GetPassiveUsage(ctx, account.ID)
	} else {
		usage, err = s.usageService.GetUsageForAccount(ctx, account, false)
	}
	if err != nil {
		slog.Warn("user_group_account_usage_failed", "user_id", userID, "account_id", account.ID, "source", source, "error", err)
		return nil, ErrUserGroupAccountUsage
	}
	return userGroupAccountUsageView(usage), nil
}

func accountInGroups(account *Account, groups []*Group) bool {
	for _, groupID := range account.GroupIDs {
		for _, group := range groups {
			if group.ID == groupID {
				return true
			}
		}
	}
	return false
}

func userGroupAccountView(account *Account, now time.Time) UserGroupAccount {
	status, until := userGroupAccountStatus(account, now)
	return UserGroupAccount{
		ID:           account.ID,
		Name:         account.Name,
		Platform:     account.Platform,
		Type:         account.Type,
		Status:       status,
		StatusUntil:  until,
		PassiveUsage: supportsAnthropicPassiveUsage(account),
	}
}

func userGroupAccountStatus(account *Account, now time.Time) (string, *time.Time) {
	future := func(t *time.Time) bool { return t != nil && t.After(now) }
	switch {
	case account.Status == StatusDisabled:
		return UserGroupAccountStatusDisabled, nil
	case account.Status == StatusError:
		return UserGroupAccountStatusError, nil
	case account.ExpiresAt != nil && !account.ExpiresAt.After(now):
		return UserGroupAccountStatusExpired, nil
	case !account.Schedulable:
		return UserGroupAccountStatusPaused, nil
	case future(account.TempUnschedulableUntil):
		return UserGroupAccountStatusTempUnschedulable, account.TempUnschedulableUntil
	case future(account.RateLimitResetAt):
		return UserGroupAccountStatusRateLimited, account.RateLimitResetAt
	case future(account.OverloadUntil):
		return UserGroupAccountStatusOverloaded, account.OverloadUntil
	default:
		return UserGroupAccountStatusActive, nil
	}
}

func userGroupAccountUsageView(usage *UsageInfo) *UserGroupAccountUsage {
	if usage == nil {
		return &UserGroupAccountUsage{}
	}
	return &UserGroupAccountUsage{
		Source:         usage.Source,
		UpdatedAt:      usage.UpdatedAt,
		FiveHour:       userGroupAccountWindow(usage.FiveHour),
		SevenDay:       userGroupAccountWindow(usage.SevenDay),
		SevenDaySonnet: userGroupAccountWindow(usage.SevenDaySonnet),
		SevenDayFable:  userGroupAccountWindow(usage.SevenDayFable),
	}
}

func userGroupAccountWindow(p *UsageProgress) *UserGroupAccountUsageWindow {
	if p == nil {
		return nil
	}
	return &UserGroupAccountUsageWindow{
		Utilization:      p.Utilization,
		ResetsAt:         p.ResetsAt,
		RemainingSeconds: p.RemainingSeconds,
	}
}
