package models

import "time"

// User 后台用户（本平台仅 admin 一个管理员角色）。
type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex" json:"username"`
	PasswordHash string    `gorm:"size:255" json:"-"`
	DisplayName  string    `gorm:"size:64" json:"display_name"`
	CreatedAt    time.Time `json:"created_at"`
}

// Venue 体育场馆。
type Venue struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128" json:"name"`
	SportType   string    `gorm:"size:32" json:"sport_type"` // basketball / football / badminton / swimming ...
	Capacity    int       `json:"capacity"`
	HourlyPrice float64   `json:"hourly_price"`
	OpenHour    int       `json:"open_hour"`  // 开放起始小时，0-23
	CloseHour   int       `json:"close_hour"` // 关闭小时，1-24
	Status      string    `gorm:"size:16" json:"status"` // open / closed / maintenance
	CreatedAt   time.Time `json:"created_at"`
}

// Booking 场地预订。
type Booking struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	VenueID      uint      `gorm:"index" json:"venue_id"`
	CustomerName string    `gorm:"size:64" json:"customer_name"`
	Phone        string    `gorm:"size:32" json:"phone"`
	BookDate     string    `gorm:"size:10;index" json:"book_date"` // YYYY-MM-DD
	StartHour    int       `json:"start_hour"`
	EndHour      int       `json:"end_hour"`
	Amount       float64   `json:"amount"`
	Status       string    `gorm:"size:16" json:"status"` // booked / cancelled / completed / rescheduled
	OriginalID   *uint     `gorm:"index" json:"original_id,omitempty"` // 改约来源预订ID
	RescheduleNote string  `gorm:"size:255" json:"reschedule_note,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// MaintenanceType 维护类型。
type MaintenanceType string

const (
	MaintenanceTypeRoutine  MaintenanceType = "routine"  // 日常保养
	MaintenanceTypeRepair   MaintenanceType = "repair"   // 临时维修
	MaintenanceTypeUpgrade  MaintenanceType = "upgrade"  // 设施升级
	MaintenanceTypeEmergency MaintenanceType = "emergency" // 紧急维修
)

// MaintenanceStatus 维护状态。
type MaintenanceStatus string

const (
	MaintenanceStatusPlanned   MaintenanceStatus = "planned"    // 计划中
	MaintenanceStatusInProgress MaintenanceStatus = "in_progress" // 进行中
	MaintenanceStatusCompleted MaintenanceStatus = "completed"  // 已完成
	MaintenanceStatusCancelled MaintenanceStatus = "cancelled"  // 已取消
)

// MaintenancePriority 维护优先级。
type MaintenancePriority string

const (
	MaintenancePriorityLow      MaintenancePriority = "low"      // 低
	MaintenancePriorityMedium   MaintenancePriority = "medium"   // 中
	MaintenancePriorityHigh     MaintenancePriority = "high"     // 高
	MaintenancePriorityUrgent   MaintenancePriority = "urgent"   // 紧急（强占）
)

// MaintenanceRule 周期维护规则（模板），用于自动展开成多次维护任务。
type MaintenanceRule struct {
	ID             uint              `gorm:"primaryKey" json:"id"`
	VenueID        uint              `gorm:"index" json:"venue_id"`
	Title          string            `gorm:"size:128" json:"title"`
	Type           MaintenanceType   `gorm:"size:32" json:"type"`
	Priority       MaintenancePriority `gorm:"size:16" json:"priority"`
	Assignee       string            `gorm:"size:64" json:"assignee"` // 负责人
	Description    string            `gorm:"type:text" json:"description,omitempty"`

	// 周期规则
	RecurrenceType string `gorm:"size:16" json:"recurrence_type"` // daily / weekly / monthly / yearly
	RecurrenceInterval int `json:"recurrence_interval"` // 间隔数，如每周=1，每两周=2

	// 针对 weekly：周几 1-7 (周一到周日)，逗号分隔，如 "1,3,5"
	Weekdays string `gorm:"size:32" json:"weekdays,omitempty"`
	// 针对 monthly/yearly：日期，如 "15" 每月15号
	MonthDay int `json:"month_day,omitempty"`

	// 单次维护的时间段（小时）
	StartHour    int `json:"start_hour"`
	EndHour      int `json:"end_hour"`
	DurationHours int `json:"duration_hours"`

	StartDate    string `gorm:"size:10" json:"start_date"` // 规则生效起始日期 YYYY-MM-DD
	EndDate      string `gorm:"size:10" json:"end_date"`   // 规则生效结束日期 YYYY-MM-DD

	Status       string `gorm:"size:16" json:"status"` // active / inactive
	CreatedAt    time.Time `json:"created_at"`
}

// MaintenanceTask 维护任务（单次具体排期）。
type MaintenanceTask struct {
	ID             uint              `gorm:"primaryKey" json:"id"`
	RuleID         *uint             `gorm:"index" json:"rule_id,omitempty"` // 来源周期规则ID
	VenueID        uint              `gorm:"index" json:"venue_id"`
	Title          string            `gorm:"size:128" json:"title"`
	Type           MaintenanceType   `gorm:"size:32" json:"type"`
	Priority       MaintenancePriority `gorm:"size:16" json:"priority"`
	Status         MaintenanceStatus `gorm:"size:16;index" json:"status"`
	Assignee       string            `gorm:"size:64" json:"assignee"`
	Description    string            `gorm:"type:text" json:"description,omitempty"`

	MaintainDate   string `gorm:"size:10;index" json:"maintain_date"` // YYYY-MM-DD
	StartHour      int    `json:"start_hour"`
	EndHour        int    `json:"end_hour"`
	DurationHours  int    `json:"duration_hours"`

	IsPeriodic     bool   `json:"is_periodic"` // 是否来自周期规则

	// 执行记录
	ActualStartAt  *time.Time `json:"actual_start_at,omitempty"`
	ActualEndAt    *time.Time `json:"actual_end_at,omitempty"`
	ActualDuration *int       `json:"actual_duration_minutes,omitempty"` // 实际耗时（分钟）
	ResultNote     string     `gorm:"type:text" json:"result_note,omitempty"`
	ReplacedParts  string     `gorm:"size:512" json:"replaced_parts,omitempty"`
	ExecutorName   string     `gorm:"size:64" json:"executor_name,omitempty"`

	CreatedAt      time.Time `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

// MaintenanceConflict 维护与预订冲突记录（用于审计追踪）。
type MaintenanceConflict struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	MaintenanceTaskID uint     `gorm:"index" json:"maintenance_task_id"`
	BookingID        uint      `gorm:"index" json:"booking_id"`
	ConflictType     string    `gorm:"size:16" json:"conflict_type"` // overlap / full
	ResolutionType   string    `gorm:"size:16" json:"resolution_type,omitempty"` // reschedule_same / reschedule_other / refund / pending
	NewBookingID     *uint     `gorm:"index" json:"new_booking_id,omitempty"`
	ResolvedAt       *time.Time `json:"resolved_at,omitempty"`
	Note             string    `gorm:"size:512" json:"note,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// RescheduleSuggestion 改约建议（不存库，接口返回用）。
type RescheduleSuggestion struct {
	VenueID       uint   `json:"venue_id"`
	VenueName     string `json:"venue_name"`
	SportType     string `json:"sport_type"`
	BookDate      string `json:"book_date"`
	StartHour     int    `json:"start_hour"`
	EndHour       int    `json:"end_hour"`
	SameVenue     bool   `json:"same_venue"`      // 是否同场馆
	PriceMatch    bool   `json:"price_match"`     // 价格是否一致
	HourlyPrice   float64 `json:"hourly_price"`
	TotalPrice    float64 `json:"total_price"`
	DistanceScore int    `json:"distance_score"`  // 距原预订天数差的绝对值
}
