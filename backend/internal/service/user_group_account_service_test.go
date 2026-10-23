//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type ugaUserRepo struct{ users map[int64]*User }

func (r *ugaUserRepo) GetByID(_ context.Context, id int64) (*User, error) {
	if u, ok := r.users[id]; ok {
		return u, nil
	}
	return nil, ErrUserNotFound
}

type ugaGroupRepo struct{ groups map[int64]*Group }

func (r *ugaGroupRepo) GetByIDLite(_ context.Context, id int64) (*Group, error) {
	if g, ok := r.groups[id]; ok {
		return g, nil
	}
	return nil, ErrGroupNotFound
}

type ugaAccountRepo struct {
	accounts     []Account
	listedGroups []int64
}

func (r *ugaAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			a := r.accounts[i]
			return &a, nil
		}
	}
	return nil, ErrAccountNotFound
}

func (r *ugaAccountRepo) ListAllWithFilters(_ context.Context, _, _, status, _ string, groupID int64, _ string) ([]Account, error) {
	r.listedGroups = append(r.listedGroups, groupID)
	out := []Account{}
	for _, a := range r.accounts {
		for _, gid := range a.GroupIDs {
			if gid == groupID && (status == "" || a.Status == status) {
				out = append(out, a)
			}
		}
	}
	return out, nil
}

type ugaUsageReader struct {
	passiveCalls []int64
	activeCalls  []int64
	activeForce  []bool
	usage        *UsageInfo
	err          error
}

func (r *ugaUsageReader) GetPassiveUsage(_ context.Context, accountID int64) (*UsageInfo, error) {
	r.passiveCalls = append(r.passiveCalls, accountID)
	return r.usage, r.err
}

func (r *ugaUsageReader) GetUsageForAccount(_ context.Context, account *Account, force ...bool) (*UsageInfo, error) {
	r.activeCalls = append(r.activeCalls, account.ID)
	r.activeForce = append(r.activeForce, len(force) > 0 && force[0])
	return r.usage, r.err
}

var ugaNow = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

// 用户 1：分配了专属分组 10（启用）、11（停用）、公开分组 20、已删除分组 99。
// 用户 2：未分配任何分组。
func newUGAFixture() (*UserGroupAccountService, *ugaAccountRepo, *ugaUsageReader) {
	users := &ugaUserRepo{users: map[int64]*User{
		1: {ID: 1, AllowedGroups: []int64{20, 10, 11, 99, 10}},
		2: {ID: 2},
	}}
	groups := &ugaGroupRepo{groups: map[int64]*Group{
		10: {ID: 10, Name: "Claude真神", Platform: PlatformAnthropic, IsExclusive: true, Status: StatusActive},
		11: {ID: 11, Name: "停用专属", Platform: PlatformAnthropic, IsExclusive: true, Status: StatusDisabled},
		20: {ID: 20, Name: "GPT plus", Platform: PlatformOpenAI, IsExclusive: false, Status: StatusActive},
	}}
	future := ugaNow.Add(30 * time.Minute)
	accounts := &ugaAccountRepo{accounts: []Account{
		{ID: 101, Name: "b-claude", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, GroupIDs: []int64{10},
			Credentials: map[string]any{"refresh_token": "secret-rt"}},
		{ID: 102, Name: "A-claude", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, RateLimitResetAt: &future, GroupIDs: []int64{10}},
		{ID: 103, Name: "broken", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusError, ErrorMessage: "401 token revoked", GroupIDs: []int64{10}},
		{ID: 201, Name: "gpt", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, GroupIDs: []int64{20}},
		{ID: 111, Name: "in-disabled-group", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, GroupIDs: []int64{11}},
	}}
	usage := &ugaUsageReader{}
	svc := &UserGroupAccountService{
		userRepo:     users,
		groupRepo:    groups,
		accountRepo:  accounts,
		usageService: usage,
		now:          func() time.Time { return ugaNow },
	}
	return svc, accounts, usage
}

func TestUserGroupAccountService_ListGroupsOnlyActiveExclusiveAssignedGroups(t *testing.T) {
	svc, accounts, _ := newUGAFixture()

	groups, err := svc.ListGroups(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, int64(10), groups[0].ID)
	require.Equal(t, []int64{10}, accounts.listedGroups, "public / disabled / missing groups must not be queried")

	// 包含异常账号（不按 active 过滤），按名称不区分大小写排序。
	got := groups[0].Accounts
	require.Len(t, got, 3)
	require.Equal(t, []string{"A-claude", "b-claude", "broken"}, []string{got[0].Name, got[1].Name, got[2].Name})
	require.Equal(t, UserGroupAccountStatusRateLimited, got[0].Status)
	require.NotNil(t, got[0].StatusUntil)
	require.Equal(t, UserGroupAccountStatusActive, got[1].Status)
	require.True(t, got[1].PassiveUsage)
	require.Equal(t, UserGroupAccountStatusError, got[2].Status)

	empty, err := svc.ListGroups(context.Background(), 2)
	require.NoError(t, err)
	require.Empty(t, empty)
}

func TestUserGroupAccountService_ListGroupsNeverLeaksSensitiveFields(t *testing.T) {
	svc, _, _ := newUGAFixture()

	groups, err := svc.ListGroups(context.Background(), 1)
	require.NoError(t, err)
	raw, err := json.Marshal(groups)
	require.NoError(t, err)
	body := string(raw)
	for _, leaked := range []string{"secret-rt", "refresh_token", "credentials", "401 token revoked", "error_message", "proxy"} {
		require.NotContains(t, body, leaked)
	}
}

func TestUserGroupAccountService_GetUsageRejectsAccountsOutsideVisibleGroups(t *testing.T) {
	svc, _, usage := newUGAFixture()
	ctx := context.Background()

	for _, tc := range []struct {
		name      string
		userID    int64
		accountID int64
	}{
		{"public group account", 1, 201},
		{"disabled exclusive group account", 1, 111},
		{"missing account", 1, 999},
		{"user without groups", 2, 101},
		{"invalid id", 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.GetUsage(ctx, tc.userID, tc.accountID, UserGroupAccountUsageSourceActive)
			require.True(t, errors.Is(err, ErrUserGroupAccountNotFound), "got %v", err)
		})
	}
	require.Empty(t, usage.passiveCalls)
	require.Empty(t, usage.activeCalls, "upstream usage must not be queried for rejected accounts")
}

func TestUserGroupAccountService_GetUsageSourcesAndNeverForce(t *testing.T) {
	svc, _, usage := newUGAFixture()
	reset := ugaNow.Add(2 * time.Hour)
	usage.usage = &UsageInfo{
		Source:    "active",
		UpdatedAt: &ugaNow,
		FiveHour: &UsageProgress{Utilization: 42.5, ResetsAt: &reset, RemainingSeconds: 7200,
			WindowStats: &WindowStats{Requests: 9, Cost: 12.34}, UsedRequests: 9, LimitRequests: 100},
		SevenDay:    &UsageProgress{Utilization: 10},
		IsForbidden: true, ForbiddenReason: "upstream detail",
	}
	ctx := context.Background()

	_, err := svc.GetUsage(ctx, 1, 101, UserGroupAccountUsageSourcePassive)
	require.NoError(t, err)
	require.Equal(t, []int64{101}, usage.passiveCalls)

	got, err := svc.GetUsage(ctx, 1, 101, UserGroupAccountUsageSourceActive)
	require.NoError(t, err)
	require.Equal(t, []int64{101}, usage.activeCalls)
	require.Equal(t, []bool{false}, usage.activeForce, "user refresh must respect usage caches")

	require.Equal(t, 42.5, got.FiveHour.Utilization)
	require.Equal(t, &reset, got.FiveHour.ResetsAt)
	require.Equal(t, 7200, got.FiveHour.RemainingSeconds)
	require.Nil(t, got.SevenDaySonnet)

	raw, err := json.Marshal(got)
	require.NoError(t, err)
	for _, leaked := range []string{"window_stats", "cost", "used_requests", "limit_requests", "forbidden", "upstream detail"} {
		require.NotContains(t, string(raw), leaked)
	}
}

func TestUserGroupAccountService_GetUsageHidesUpstreamErrors(t *testing.T) {
	svc, _, usage := newUGAFixture()
	usage.err = errors.New("anthropic 401: invalid token for someone@example.com")

	_, err := svc.GetUsage(context.Background(), 1, 101, UserGroupAccountUsageSourceActive)
	require.True(t, errors.Is(err, ErrUserGroupAccountUsage))
	require.NotContains(t, infraerrors.FromError(err).Message, "example.com")
}

func TestUserGroupAccountStatusPriority(t *testing.T) {
	past := ugaNow.Add(-time.Minute)
	future := ugaNow.Add(time.Hour)
	later := ugaNow.Add(2 * time.Hour)
	for _, tc := range []struct {
		name      string
		account   Account
		want      string
		wantUntil *time.Time
	}{
		{"disabled wins", Account{Status: StatusDisabled, Schedulable: true, RateLimitResetAt: &future}, UserGroupAccountStatusDisabled, nil},
		{"error", Account{Status: StatusError, Schedulable: true}, UserGroupAccountStatusError, nil},
		{"expired", Account{Status: StatusActive, Schedulable: true, ExpiresAt: &past}, UserGroupAccountStatusExpired, nil},
		{"paused", Account{Status: StatusActive, Schedulable: false, RateLimitResetAt: &future}, UserGroupAccountStatusPaused, nil},
		{"temp unschedulable", Account{Status: StatusActive, Schedulable: true, TempUnschedulableUntil: &future, RateLimitResetAt: &later}, UserGroupAccountStatusTempUnschedulable, &future},
		{"rate limited", Account{Status: StatusActive, Schedulable: true, RateLimitResetAt: &future, OverloadUntil: &later}, UserGroupAccountStatusRateLimited, &future},
		{"overloaded", Account{Status: StatusActive, Schedulable: true, OverloadUntil: &future}, UserGroupAccountStatusOverloaded, &future},
		{"stale limits are active", Account{Status: StatusActive, Schedulable: true, RateLimitResetAt: &past, OverloadUntil: &past, ExpiresAt: &future}, UserGroupAccountStatusActive, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, until := userGroupAccountStatus(&tc.account, ugaNow)
			require.Equal(t, tc.want, got)
			require.Equal(t, tc.wantUntil, until)
		})
	}
}
