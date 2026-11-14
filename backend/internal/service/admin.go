package service

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"knownote/internal/config"
	"knownote/internal/db"
	"knownote/internal/llm"
	"knownote/internal/model"
	"knownote/internal/settings"
	"sync"
	"time"
)

var ErrDisabled = &Error{403, 40301, "账号已被停用"}

func lockAdmins(tx *gorm.DB) error { return tx.Exec("SELECT pg_advisory_xact_lock(482947111)").Error }
func assignAdmins(tx *gorm.DB, emails []string) error {
	if len(emails) > 0 {
		return tx.Model(&model.User{}).Where("email IN ?", emails).Update("role", "admin").Error
	}
	var count int64
	if err := tx.Model(&model.User{}).Where("role = 'admin'").Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	var first model.User
	err := tx.Order("created_at, id").First(&first).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return tx.Model(&first).Update("role", "admin").Error
}
func (s *Service) BootstrapAdmins(ctx context.Context) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockAdmins(tx); err != nil {
			return err
		}
		return assignAdmins(tx, s.AdminEmails)
	})
}

type AdminUser struct {
	UserView
	Disabled      bool  `json:"disabled"`
	NotebookCount int64 `json:"notebookCount"`
	TodayRequests int64 `json:"todayRequests"`
	TodayTokens   int64 `json:"todayTokens"`
	WeekTokens    int64 `json:"weekTokens"`
}
type UserPage struct {
	Items    []AdminUser `json:"items"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
}

var adminZone = time.FixedZone("Asia/Shanghai", 8*3600)

func adminDay() time.Time {
	now := time.Now().In(adminZone)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, adminZone)
}
func (s *Service) AdminUsers(ctx context.Context, query string, page, pageSize int) (UserPage, error) {
	result := UserPage{Items: []AdminUser{}, Page: page, PageSize: pageSize}
	q := s.DB.WithContext(ctx).Model(&model.User{})
	if query != "" {
		q = q.Where("email ILIKE ? OR nickname ILIKE ?", "%"+query+"%", "%"+query+"%")
	}
	if err := q.Count(&result.Total).Error; err != nil {
		return result, err
	}
	start := adminDay()
	week := start.AddDate(0, 0, -6)
	end := start.AddDate(0, 0, 1)
	err := q.Select(`users.*, (SELECT COUNT(*) FROM notebooks n WHERE n.user_id=users.id) AS notebook_count,
 (SELECT COUNT(*) FROM llm_usage l WHERE l.user_id=users.id AND l.capability='chat' AND l.created_at>=? AND l.created_at<?) AS today_requests,
 (SELECT COALESCE(SUM(prompt_tokens+completion_tokens),0) FROM llm_usage l WHERE l.user_id=users.id AND l.created_at>=? AND l.created_at<?) AS today_tokens,
 (SELECT COALESCE(SUM(prompt_tokens+completion_tokens),0) FROM llm_usage l WHERE l.user_id=users.id AND l.created_at>=? AND l.created_at<?) AS week_tokens`, start, end, start, end, week, end).Order("users.created_at,users.id").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&result.Items).Error
	return result, err
}
func (s *Service) UpdateAdminUser(ctx context.Context, actor, id string, disabled *bool, role *string) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockAdmins(tx); err != nil {
			return err
		}
		var admin, user model.User
		if err := tx.Where("id = ? AND role = 'admin' AND disabled = false", actor).First(&admin).Error; err != nil {
			return &Error{403, 40301, "仅管理员可访问"}
		}
		if err := tx.Where("id = ?", id).First(&user).Error; err != nil {
			return databaseError(err)
		}
		if actor == id && ((disabled != nil && *disabled) || (role != nil && *role != "admin")) {
			return &Error{400, 40001, "不能停用自己或撤销自己的管理员权限"}
		}
		nextRole, nextDisabled := user.Role, user.Disabled
		if role != nil {
			nextRole = *role
		}
		if disabled != nil {
			nextDisabled = *disabled
		}
		if user.Role == "admin" && !user.Disabled && (nextRole != "admin" || nextDisabled) {
			var count int64
			if err := tx.Model(&model.User{}).Where("role = 'admin' AND disabled = false AND id <> ?", id).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return &Error{400, 40001, "系统必须保留至少一位可用管理员"}
			}
		}
		if err := tx.Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{"role": nextRole, "disabled": nextDisabled}).Error; err != nil {
			return err
		}
		if user.Disabled != nextDisabled {
			action := "启用用户"
			if nextDisabled {
				action = "停用用户"
			}
			if err := settings.Audit(tx, actor, action, id, "账号状态已更新"); err != nil {
				return err
			}
		}
		if user.Role != nextRole {
			if err := settings.Audit(tx, actor, "变更角色", id, "角色设为 "+nextRole); err != nil {
				return err
			}
		}
		return nil
	})
}

type UsageDay struct {
	Date     string `json:"date"`
	Requests int64  `json:"requests"`
	Tokens   int64  `json:"tokens"`
}
type UsageTop struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Nickname string `json:"nickname"`
	Requests int64  `json:"requests"`
	Tokens   int64  `json:"tokens"`
}
type AdminUsage struct {
	Days     []UsageDay `json:"days"`
	TopUsers []UsageTop `json:"topUsers"`
}

func (s *Service) AdminUsage(ctx context.Context) (AdminUsage, error) {
	start := adminDay()
	end := start.AddDate(0, 0, 1)
	week := start.AddDate(0, 0, -6)
	result := AdminUsage{Days: []UsageDay{}, TopUsers: []UsageTop{}}
	var rows []UsageDay
	err := s.DB.WithContext(ctx).Model(&model.LLMUsage{}).Where("created_at >= ? AND created_at < ?", week, end).Select("to_char(created_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD') AS date,COUNT(*) AS requests,COALESCE(SUM(prompt_tokens+completion_tokens),0) AS tokens").Group("date").Scan(&rows).Error
	if err != nil {
		return result, err
	}
	byDate := map[string]UsageDay{}
	for _, row := range rows {
		byDate[row.Date] = row
	}
	for i := 0; i < 7; i++ {
		day := week.AddDate(0, 0, i).Format("2006-01-02")
		row := byDate[day]
		row.Date = day
		result.Days = append(result.Days, row)
	}
	err = s.DB.WithContext(ctx).Table("llm_usage l").Joins("JOIN users u ON u.id=l.user_id").Where("l.created_at>=? AND l.created_at<?", start, end).Select("u.id,u.email,u.nickname,COUNT(*) AS requests,SUM(l.prompt_tokens+l.completion_tokens) AS tokens").Group("u.id").Order("tokens DESC,u.id").Limit(10).Scan(&result.TopUsers).Error
	return result, err
}
func (s *Service) AuditLogs(ctx context.Context) ([]model.AuditLog, error) {
	rows := []model.AuditLog{}
	err := s.DB.WithContext(ctx).Order("created_at DESC,id DESC").Limit(50).Find(&rows).Error
	return rows, err
}

type ReindexProgress struct {
	Total     int64  `json:"total"`
	Completed int64  `json:"completed"`
	Failed    int64  `json:"failed"`
	Running   bool   `json:"running"`
	Message   string `json:"message"`
}
type AdminService struct {
	Settings *settings.Manager
	DB       *gorm.DB
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	mu       sync.Mutex
	progress ReindexProgress
}

func NewAdminService(ctx context.Context, database *gorm.DB, m *settings.Manager) *AdminService {
	ctx, cancel := context.WithCancel(ctx)
	a := &AdminService{Settings: m, DB: database, ctx: ctx, cancel: cancel}
	m.VectorBusy = func() bool { return a.Progress().Running }
	return a
}
func (a *AdminService) Close() { a.cancel(); a.wg.Wait() }
func (a *AdminService) Progress() ReindexProgress {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.progress
}
func (a *AdminService) Start(ctx context.Context, userID string) error {
	return a.Settings.WithSnapshot(func(snapshot llm.Snapshot) error {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.progress.Running {
			return &Error{409, 40901, "已有向量重建任务正在运行"}
		}
		if err := settings.Audit(a.DB.WithContext(ctx), userID, "启动重建", "chunks", "使用当前向量配置重建全部分块"); err != nil {
			return err
		}
		a.progress = ReindexProgress{Running: true, Message: "重建中，资料读写可能等待；失败时全部回滚"}
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			guard := llm.NewGuard(a.DB, snapshot.Client, snapshot.Config)
			guard.Limits = func() config.Config { return a.Settings.Provider.Current().Config }
			err := db.ReindexWithProgress(a.ctx, a.DB, guard, snapshot.Config.EmbeddingDim, func(total, completed int64) {
				a.mu.Lock()
				a.progress.Total, a.progress.Completed = total, completed
				a.mu.Unlock()
			})
			if err == nil {
				a.Settings.ReindexDone()
			}
			a.mu.Lock()
			defer a.mu.Unlock()
			a.progress.Running = false
			if err != nil {
				a.progress.Failed = max(1, a.progress.Total)
				a.progress.Completed = 0
				a.progress.Message = "重建失败，已全部回滚，请检查配置、额度后重试"
			} else {
				a.progress.Message = "全部向量已重建"
			}
		}()
		return nil
	})
}
