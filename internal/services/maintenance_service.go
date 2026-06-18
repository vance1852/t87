package services

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"venue-booking-admin/internal/models"
)

const (
	dateFormat = "2006-01-02"
	searchDays = 30 // 改约建议搜索未来30天
)

// MaintenanceService 维护核心服务。
type MaintenanceService struct {
	DB *gorm.DB
}

// NewMaintenanceService 构造函数。
func NewMaintenanceService(db *gorm.DB) *MaintenanceService {
	return &MaintenanceService{DB: db}
}

// ============================================================
// 周期规则展开
// ============================================================

// ExpandMaintenanceRule 根据周期规则展开成 MaintenanceTask 列表（不入库）。
func (s *MaintenanceService) ExpandMaintenanceRule(rule *models.MaintenanceRule) ([]models.MaintenanceTask, error) {
	if rule.StartDate == "" || rule.EndDate == "" {
		return nil, fmt.Errorf("周期规则必须指定起止日期")
	}
	start, err := time.Parse(dateFormat, rule.StartDate)
	if err != nil {
		return nil, fmt.Errorf("起始日期格式错误: %w", err)
	}
	end, err := time.Parse(dateFormat, rule.EndDate)
	if err != nil {
		return nil, fmt.Errorf("结束日期格式错误: %w", err)
	}
	if end.Before(start) {
		return nil, fmt.Errorf("结束日期不能早于起始日期")
	}
	if rule.DurationHours <= 0 {
		rule.DurationHours = rule.EndHour - rule.StartHour
	}
	interval := rule.RecurrenceInterval
	if interval <= 0 {
		interval = 1
	}

	var tasks []models.MaintenanceTask

	switch rule.RecurrenceType {
	case "daily":
		for d := start; !d.After(end); d = d.AddDate(0, 0, interval) {
			tasks = append(tasks, s.buildTaskFromRule(rule, d))
		}
	case "weekly":
		weekdays := parseWeekdays(rule.Weekdays)
		if len(weekdays) == 0 {
			// 默认规则创建当天的周几
			weekdays = []int{int(start.Weekday())}
			if weekdays[0] == 0 {
				weekdays[0] = 7
			}
		}
		for d := start; !d.After(end); d = d.AddDate(0, 0, 7*interval) {
			// 从当前周的周一开始扫描这一周
			wd := int(d.Weekday())
			if wd == 0 {
				wd = 7
			}
			monday := d.AddDate(0, 0, 1-wd)
			for _, targetWd := range weekdays {
				candidate := monday.AddDate(0, 0, targetWd-1)
				if (candidate.After(start) || candidate.Equal(start)) && !candidate.After(end) {
					tasks = append(tasks, s.buildTaskFromRule(rule, candidate))
				}
			}
		}
	case "monthly":
		day := rule.MonthDay
		if day <= 0 {
			day = start.Day()
		}
		for y, m := start.Year(), int(start.Month()); ; {
			candidate := makeDate(y, m, day)
			if candidate.After(end) {
				break
			}
			if !candidate.Before(start) {
				tasks = append(tasks, s.buildTaskFromRule(rule, candidate))
			}
			m += interval
			for m > 12 {
				m -= 12
				y++
			}
			if y > end.Year()+5 {
				break
			}
		}
	case "yearly":
		day := rule.MonthDay
		if day <= 0 {
			day = start.Day()
		}
		month := int(start.Month())
		for y := start.Year(); y <= end.Year(); y += interval {
			candidate := makeDate(y, month, day)
			if candidate.After(end) {
				break
			}
			if !candidate.Before(start) {
				tasks = append(tasks, s.buildTaskFromRule(rule, candidate))
			}
		}
	default:
		return nil, fmt.Errorf("不支持的周期类型: %s", rule.RecurrenceType)
	}
	return tasks, nil
}

// CreateMaintenanceRuleAndExpand 创建规则并立即展开生成任务（已存在同日同时段同场馆的任务则跳过）。
func (s *MaintenanceService) CreateMaintenanceRuleAndExpand(rule *models.MaintenanceRule) (*models.MaintenanceRule, []models.MaintenanceTask, error) {
	rule.Status = "active"
	if err := s.DB.Create(rule).Error; err != nil {
		return nil, nil, err
	}
	tasks, err := s.ExpandMaintenanceRule(rule)
	if err != nil {
		return rule, nil, err
	}
	ruleID := rule.ID
	var created []models.MaintenanceTask
	for i := range tasks {
		t := tasks[i]
		t.RuleID = &ruleID
		t.IsPeriodic = true
		var exists int64
		s.DB.Model(&models.MaintenanceTask{}).
			Where("venue_id = ? AND maintain_date = ? AND start_hour = ? AND end_hour = ? AND status <> ?",
				t.VenueID, t.MaintainDate, t.StartHour, t.EndHour, models.MaintenanceStatusCancelled).
			Count(&exists)
		if exists == 0 {
			if err := s.DB.Create(&t).Error; err == nil {
				created = append(created, t)
			}
		}
	}
	return rule, created, nil
}

func (s *MaintenanceService) buildTaskFromRule(rule *models.MaintenanceRule, d time.Time) models.MaintenanceTask {
	return models.MaintenanceTask{
		VenueID:       rule.VenueID,
		Title:         rule.Title,
		Type:          rule.Type,
		Priority:      rule.Priority,
		Status:        models.MaintenanceStatusPlanned,
		Assignee:      rule.Assignee,
		Description:   rule.Description,
		MaintainDate:  d.Format(dateFormat),
		StartHour:     rule.StartHour,
		EndHour:       rule.EndHour,
		DurationHours: rule.EndHour - rule.StartHour,
		IsPeriodic:    true,
	}
}

// ============================================================
// 时段冲突检测
// ============================================================

// CheckBookingVsMaintenance 预订创建前校验：同场馆是否有未取消的维护占用时段。
func (s *MaintenanceService) CheckBookingVsMaintenance(venueID uint, bookDate string, startHour, endHour int) ([]models.MaintenanceTask, error) {
	var tasks []models.MaintenanceTask
	err := s.DB.Where("venue_id = ? AND maintain_date = ? AND status <> ?",
		venueID, bookDate, models.MaintenanceStatusCancelled).
		Where("start_hour < ? AND end_hour > ?", endHour, startHour).
		Find(&tasks).Error
	return tasks, err
}

// CheckMaintenanceVsBookings 维护排期时检测：找出重叠的活跃预订。
func (s *MaintenanceService) CheckMaintenanceVsBookings(venueID uint, maintainDate string, startHour, endHour int) ([]models.Booking, error) {
	var bookings []models.Booking
	err := s.DB.Where("venue_id = ? AND book_date = ? AND status NOT IN ?",
		venueID, maintainDate, []string{"cancelled", "completed", "rescheduled"}).
		Where("start_hour < ? AND end_hour > ?", endHour, startHour).
		Find(&bookings).Error
	return bookings, err
}

// CreateMaintenanceTaskWithConflictCheck 创建维护任务并返回冲突信息。
func (s *MaintenanceService) CreateMaintenanceTaskWithConflictCheck(task *models.MaintenanceTask) (*ConflictDetectResult, error) {
	if task.Status == "" {
		task.Status = models.MaintenanceStatusPlanned
	}
	if task.DurationHours <= 0 {
		task.DurationHours = task.EndHour - task.StartHour
	}

	// 1. 检测同场馆是否有其他维护冲突（未取消）
	var existingTasks []models.MaintenanceTask
	s.DB.Where("venue_id = ? AND maintain_date = ? AND status <> ? AND id <> ?",
		task.VenueID, task.MaintainDate, models.MaintenanceStatusCancelled, 0).
		Where("start_hour < ? AND end_hour > ?", task.EndHour, task.StartHour).
		Find(&existingTasks)
	if len(existingTasks) > 0 {
		return nil, fmt.Errorf("该场馆此时段已有未取消的维护计划")
	}

	// 2. 检测预订冲突
	bookings, err := s.CheckMaintenanceVsBookings(task.VenueID, task.MaintainDate, task.StartHour, task.EndHour)
	if err != nil {
		return nil, err
	}

	// 3. 优先写入任务（冲突可以后续再处理，尤其是紧急维护）
	if err := s.DB.Create(task).Error; err != nil {
		return nil, err
	}

	// 4. 生成冲突记录（pending 状态）
	result := &ConflictDetectResult{
		TaskID:       task.ID,
		HasConflict:  len(bookings) > 0,
		ConflictCount: len(bookings),
		Bookings:     bookings,
	}

	for _, b := range bookings {
		conflict := models.MaintenanceConflict{
			MaintenanceTaskID: task.ID,
			BookingID:         b.ID,
			ConflictType:      classifyConflict(b.StartHour, b.EndHour, task.StartHour, task.EndHour),
			ResolutionType:    "pending",
		}
		s.DB.Create(&conflict)
	}

	// 5. 生成改约建议（针对每个冲突预订）
	if len(bookings) > 0 {
		suggestionsMap := make(map[uint][]models.RescheduleSuggestion)
		for _, b := range bookings {
			venue, _ := s.getVenue(b.VenueID)
			sugs, _ := s.GenerateRescheduleSuggestions(b, searchDays)
			suggestionsMap[b.ID] = sugs
			_ = venue
		}
		result.SuggestionsByBooking = suggestionsMap

		// 紧急维护：如果是 urgent 优先级，自动触发批量改约/退订
		if task.Priority == models.MaintenancePriorityUrgent {
			autoResult, _ := s.BatchResolveConflicts(task.ID, ResolveStrategy{Auto: true})
			result.AutoResolveResult = autoResult
		}
	}

	return result, nil
}

// ============================================================
// 改约建议生成
// ============================================================

// GenerateRescheduleSuggestions 为一个受影响预订生成改约建议。
// 优先级：同场馆同时段最近日期 > 同场馆其他时段 > 同类场馆同时段 > 同类场馆其他时段
func (s *MaintenanceService) GenerateRescheduleSuggestions(booking models.Booking, maxDays int) ([]models.RescheduleSuggestion, error) {
	duration := booking.EndHour - booking.StartHour
	if duration <= 0 {
		duration = 1
	}
	bookDate, _ := time.Parse(dateFormat, booking.BookDate)

	// 1. 获取原场馆和同类场馆列表
	var originalVenue models.Venue
	if err := s.DB.First(&originalVenue, booking.VenueID).Error; err != nil {
		return nil, err
	}
	var sameTypeVenues []models.Venue
	s.DB.Where("sport_type = ? AND status = ? AND id <> ?", originalVenue.SportType, "open", originalVenue.ID).
		Find(&sameTypeVenues)
	venuesToCheck := []models.Venue{originalVenue}
	venuesToCheck = append(venuesToCheck, sameTypeVenues...)

	var suggestions []models.RescheduleSuggestion
	today := time.Now()
	if bookDate.Before(today) {
		bookDate = today
	}

	// 搜索从今天到未来 maxDays 天
	for dayOffset := 0; dayOffset <= maxDays; dayOffset++ {
		candidateDate := bookDate.AddDate(0, 0, dayOffset)
		dateStr := candidateDate.Format(dateFormat)

		for _, v := range venuesToCheck {
			// 生成候选时段：优先原时段，然后逐小时尝试
			candidateHours := buildCandidateHours(booking.StartHour, booking.EndHour, v.OpenHour, v.CloseHour)

			for _, ch := range candidateHours {
				cStart, cEnd := ch[0], ch[1]
				if cEnd-cStart != duration {
					continue
				}
				if cStart < v.OpenHour || cEnd > v.CloseHour {
					continue
				}

				// 检查：是否已有预订
				var bkCount int64
				s.DB.Model(&models.Booking{}).
					Where("venue_id = ? AND book_date = ? AND status NOT IN ?",
						v.ID, dateStr, []string{"cancelled", "rescheduled"}).
					Where("start_hour < ? AND end_hour > ?", cEnd, cStart).
					Count(&bkCount)
				if bkCount > 0 {
					continue
				}

				// 检查：是否有维护
				var mtCount int64
				s.DB.Model(&models.MaintenanceTask{}).
					Where("venue_id = ? AND maintain_date = ? AND status <> ?",
						v.ID, dateStr, models.MaintenanceStatusCancelled).
					Where("start_hour < ? AND end_hour > ?", cEnd, cStart).
					Count(&mtCount)
				if mtCount > 0 {
					continue
				}

				totalPrice := v.HourlyPrice * float64(duration)
				distance := dayOffset
				if dateStr != booking.BookDate {
					// 计算日期差
					t1, _ := time.Parse(dateFormat, booking.BookDate)
					t2, _ := time.Parse(dateFormat, dateStr)
					distance = int(math.Abs(float64(int(t2.Sub(t1).Hours() / 24))))
				}

				suggestions = append(suggestions, models.RescheduleSuggestion{
					VenueID:       v.ID,
					VenueName:     v.Name,
					SportType:     v.SportType,
					BookDate:      dateStr,
					StartHour:     cStart,
					EndHour:       cEnd,
					SameVenue:     v.ID == booking.VenueID,
					PriceMatch:    v.HourlyPrice == originalVenue.HourlyPrice,
					HourlyPrice:   v.HourlyPrice,
					TotalPrice:    totalPrice,
					DistanceScore: distance,
				})

				if len(suggestions) >= 20 {
					goto doneSearch
				}
			}
		}
	}
doneSearch:

	// 排序：同场馆优先 > 价格一致优先 > 日期最近优先 > 原时段优先
	sort.SliceStable(suggestions, func(i, j int) bool {
		if suggestions[i].SameVenue != suggestions[j].SameVenue {
			return suggestions[i].SameVenue
		}
		if suggestions[i].PriceMatch != suggestions[j].PriceMatch {
			return suggestions[i].PriceMatch
		}
		if suggestions[i].DistanceScore != suggestions[j].DistanceScore {
			return suggestions[i].DistanceScore < suggestions[j].DistanceScore
		}
		return suggestions[i].StartHour < suggestions[j].StartHour
	})

	return suggestions, nil
}

// ============================================================
// 冲突解决：改约 / 退订
// ============================================================

// ResolveStrategy 批量解决策略。
type ResolveStrategy struct {
	Auto       bool   // 自动模式：首选同场馆改约，否则退款
	ForceRefund bool   // 强制全部退款
}

// ResolveResult 解决结果。
type ResolveResult struct {
	TaskID            uint             `json:"task_id"`
	TotalAffected     int              `json:"total_affected"`
	RescheduledSame   int              `json:"rescheduled_same"`
	RescheduledOther  int              `json:"rescheduled_other"`
	Refunded          int              `json:"refunded"`
	Failed            int              `json:"failed"`
	NewBookings       []models.Booking `json:"new_bookings,omitempty"`
	CancelledBookings []uint           `json:"cancelled_booking_ids,omitempty"`
	FailedBookings    []uint           `json:"failed_booking_ids,omitempty"`
}

// ConflictDetectResult 创建维护任务时的冲突检测结果。
type ConflictDetectResult struct {
	TaskID               uint                                  `json:"task_id"`
	HasConflict          bool                                  `json:"has_conflict"`
	ConflictCount        int                                   `json:"conflict_count"`
	Bookings             []models.Booking                      `json:"conflict_bookings,omitempty"`
	SuggestionsByBooking map[uint][]models.RescheduleSuggestion `json:"suggestions_by_booking,omitempty"`
	AutoResolveResult    *ResolveResult                        `json:"auto_resolve_result,omitempty"`
}

// ResolveSingleConflict 处理单个冲突预订：改约到指定建议或退款。
func (s *MaintenanceService) ResolveSingleConflict(
	taskID uint,
	bookingID uint,
	suggestion *models.RescheduleSuggestion, // nil 表示退款
) (*models.Booking, error) {
	var booking models.Booking
	if err := s.DB.First(&booking, bookingID).Error; err != nil {
		return nil, fmt.Errorf("预订不存在")
	}
	var conflict models.MaintenanceConflict
	if err := s.DB.Where("maintenance_task_id = ? AND booking_id = ?", taskID, bookingID).
		First(&conflict).Error; err != nil {
		return nil, fmt.Errorf("冲突记录不存在")
	}

	tx := s.DB.Begin()

	if suggestion != nil {
		// 改约
		newBooking := models.Booking{
			VenueID:        suggestion.VenueID,
			CustomerName:   booking.CustomerName,
			Phone:          booking.Phone,
			BookDate:       suggestion.BookDate,
			StartHour:      suggestion.StartHour,
			EndHour:        suggestion.EndHour,
			Amount:         suggestion.TotalPrice,
			Status:         "booked",
			OriginalID:     &booking.ID,
			RescheduleNote: fmt.Sprintf("因维护任务#%d改约", taskID),
		}
		if err := tx.Create(&newBooking).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		// 原预订标记为已改约
		booking.Status = "rescheduled"
		booking.RescheduleNote = fmt.Sprintf("改约至预订#%d", newBooking.ID)
		if err := tx.Save(&booking).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		// 更新冲突记录
		resType := "reschedule_same"
		if !suggestion.SameVenue {
			resType = "reschedule_other"
		}
		now := time.Now()
		conflict.ResolutionType = resType
		conflict.NewBookingID = &newBooking.ID
		conflict.ResolvedAt = &now
		if err := tx.Save(&conflict).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		tx.Commit()
		return &newBooking, nil
	}

	// 退款：标记原预订为取消
	booking.Status = "cancelled"
	booking.RescheduleNote = fmt.Sprintf("因维护任务#%d取消并退款", taskID)
	if err := tx.Save(&booking).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	now := time.Now()
	conflict.ResolutionType = "refund"
	conflict.ResolvedAt = &now
	conflict.Note = fmt.Sprintf("退款金额: %.2f", booking.Amount)
	if err := tx.Save(&conflict).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	tx.Commit()
	return nil, nil
}

// BatchResolveConflicts 批量处理某个维护任务下的所有冲突预订。
func (s *MaintenanceService) BatchResolveConflicts(taskID uint, strategy ResolveStrategy) (*ResolveResult, error) {
	var task models.MaintenanceTask
	if err := s.DB.First(&task, taskID).Error; err != nil {
		return nil, fmt.Errorf("维护任务不存在")
	}

	var conflicts []models.MaintenanceConflict
	s.DB.Where("maintenance_task_id = ? AND (resolution_type IS NULL OR resolution_type = ?)", taskID, "pending").
		Find(&conflicts)

	result := &ResolveResult{TaskID: taskID, TotalAffected: len(conflicts)}

	for _, c := range conflicts {
		var booking models.Booking
		if err := s.DB.First(&booking, c.BookingID).Error; err != nil {
			result.Failed++
			result.FailedBookings = append(result.FailedBookings, c.BookingID)
			continue
		}

		if strategy.ForceRefund {
			_, err := s.ResolveSingleConflict(taskID, c.BookingID, nil)
			if err == nil {
				result.Refunded++
				result.CancelledBookings = append(result.CancelledBookings, c.BookingID)
			} else {
				result.Failed++
				result.FailedBookings = append(result.FailedBookings, c.BookingID)
			}
			continue
		}

		// 自动模式：找改约建议，优先同场馆
		sugs, err := s.GenerateRescheduleSuggestions(booking, searchDays)
		if err != nil || len(sugs) == 0 {
			// 无可用时段，退款
			_, e := s.ResolveSingleConflict(taskID, c.BookingID, nil)
			if e == nil {
				result.Refunded++
				result.CancelledBookings = append(result.CancelledBookings, c.BookingID)
			} else {
				result.Failed++
				result.FailedBookings = append(result.FailedBookings, c.BookingID)
			}
			continue
		}
		best := &sugs[0]
		newBk, e := s.ResolveSingleConflict(taskID, c.BookingID, best)
		if e == nil {
			if best.SameVenue {
				result.RescheduledSame++
			} else {
				result.RescheduledOther++
			}
			if newBk != nil {
				result.NewBookings = append(result.NewBookings, *newBk)
			}
		} else {
			result.Failed++
			result.FailedBookings = append(result.FailedBookings, c.BookingID)
		}
	}
	return result, nil
}

// ============================================================
// 维护任务状态流转 / 执行记录
// ============================================================

// UpdateMaintenanceStatus 状态流转，附带执行记录。
func (s *MaintenanceService) UpdateMaintenanceStatus(taskID uint, newStatus models.MaintenanceStatus, executorNote string, executor string) (*models.MaintenanceTask, error) {
	var task models.MaintenanceTask
	if err := s.DB.First(&task, taskID).Error; err != nil {
		return nil, fmt.Errorf("维护任务不存在")
	}

	now := time.Now()
	switch newStatus {
	case models.MaintenanceStatusInProgress:
		if task.Status != models.MaintenanceStatusPlanned {
			return nil, fmt.Errorf("只能从planned状态进入进行中")
		}
		task.ActualStartAt = &now
		if executor != "" {
			task.ExecutorName = executor
		}
	case models.MaintenanceStatusCompleted:
		if task.Status != models.MaintenanceStatusInProgress {
			return nil, fmt.Errorf("只能从in_progress状态标记完成")
		}
		task.ActualEndAt = &now
		task.CompletedAt = &now
		if task.ActualStartAt != nil {
			mins := int(now.Sub(*task.ActualStartAt).Minutes())
			task.ActualDuration = &mins
		}
		if executorNote != "" {
			task.ResultNote = executorNote
		}
	case models.MaintenanceStatusCancelled:
		// 取消维护：需要恢复可订（无需特殊操作，因为维护表就是事实来源，预订系统通过 status!=cancelled 判断）
		if task.Status == models.MaintenanceStatusCompleted {
			return nil, fmt.Errorf("已完成的任务无法取消")
		}
	case models.MaintenanceStatusPlanned:
		// 恢复为计划中：仅当从 cancelled
		if task.Status != models.MaintenanceStatusCancelled {
			return nil, fmt.Errorf("只能从cancelled恢复为planned")
		}
	}
	task.Status = newStatus
	if err := s.DB.Save(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// ============================================================
// 维护日历 & 空档
// ============================================================

// GetMaintenanceCalendar 按日期范围返回维护占用。
func (s *MaintenanceService) GetMaintenanceCalendar(venueID *uint, startDate, endDate string, status []string) ([]models.MaintenanceTask, error) {
	q := s.DB.Model(&models.MaintenanceTask{})
	if venueID != nil && *venueID > 0 {
		q = q.Where("venue_id = ?", *venueID)
	}
	if startDate != "" {
		q = q.Where("maintain_date >= ?", startDate)
	}
	if endDate != "" {
		q = q.Where("maintain_date <= ?", endDate)
	}
	if len(status) > 0 {
		q = q.Where("status IN ?", status)
	}
	var tasks []models.MaintenanceTask
	err := q.Order("maintain_date, start_hour").Find(&tasks).Error
	return tasks, err
}

// GetVenueFreeSlots 查询场馆在指定日期的空闲时段（排除维护和已订）。
type FreeSlot struct {
	Date      string `json:"date"`
	StartHour int    `json:"start_hour"`
	EndHour   int    `json:"end_hour"`
}

func (s *MaintenanceService) GetVenueFreeSlots(venueID uint, dateStr string) ([]FreeSlot, error) {
	var venue models.Venue
	if err := s.DB.First(&venue, venueID).Error; err != nil {
		return nil, err
	}

	// 获取占用的时段（合并预订和维护）
	type occupied struct {
		StartHour int
		EndHour   int
	}
	var occ []occupied

	s.DB.Model(&models.Booking{}).
		Select("start_hour, end_hour").
		Where("venue_id = ? AND book_date = ? AND status NOT IN ?",
			venueID, dateStr, []string{"cancelled", "rescheduled"}).
		Scan(&occ)

	var mtOcc []occupied
	s.DB.Model(&models.MaintenanceTask{}).
		Select("start_hour, end_hour").
		Where("venue_id = ? AND maintain_date = ? AND status <> ?",
			venueID, dateStr, models.MaintenanceStatusCancelled).
		Scan(&mtOcc)
	occ = append(occ, mtOcc...)

	// 合并重叠区间
	sort.Slice(occ, func(i, j int) bool { return occ[i].StartHour < occ[j].StartHour })
	var merged []occupied
	for _, o := range occ {
		if len(merged) == 0 || o.StartHour >= merged[len(merged)-1].EndHour {
			merged = append(merged, o)
		} else {
			if o.EndHour > merged[len(merged)-1].EndHour {
				merged[len(merged)-1].EndHour = o.EndHour
			}
		}
	}

	// 计算空档
	var slots []FreeSlot
	cur := venue.OpenHour
	for _, m := range merged {
		if m.StartHour > cur {
			slots = append(slots, FreeSlot{Date: dateStr, StartHour: cur, EndHour: m.StartHour})
		}
		if m.EndHour > cur {
			cur = m.EndHour
		}
	}
	if cur < venue.CloseHour {
		slots = append(slots, FreeSlot{Date: dateStr, StartHour: cur, EndHour: venue.CloseHour})
	}
	return slots, nil
}

// ============================================================
// 统计分析
// ============================================================

// MaintenanceImpact 维护对营收的影响。
type MaintenanceImpact struct {
	TotalMaintenanceHours int     `json:"total_maintenance_hours"` // 维护总占用小时数
	ClosedSlots           int     `json:"closed_slots"`            // 损失可订时段数（按小时计）
	EstimatedRevenueLoss  float64 `json:"estimated_revenue_loss"`  // 估算营收损失
	CompletedCount        int64   `json:"completed_count"`
	PlannedCount          int64   `json:"planned_count"`
	InProgressCount       int64   `json:"in_progress_count"`
	CancelledCount        int64   `json:"cancelled_count"`
	TotalTasks            int64   `json:"total_tasks"`
	CompletionRate        float64 `json:"completion_rate"` // 完成率
	AvgDurationMinutes    float64 `json:"avg_duration_minutes"`
}

// GetMaintenanceStats 按日期范围统计维护影响与完成率。
func (s *MaintenanceService) GetMaintenanceStats(startDate, endDate string, venueID *uint) (*MaintenanceImpact, error) {
	q := s.DB.Model(&models.MaintenanceTask{})
	if venueID != nil && *venueID > 0 {
		q = q.Where("venue_id = ?", *venueID)
	}
	if startDate != "" {
		q = q.Where("maintain_date >= ?", startDate)
	}
	if endDate != "" {
		q = q.Where("maintain_date <= ?", endDate)
	}

	var impact MaintenanceImpact
	q.Where("status = ?", models.MaintenanceStatusCompleted).Count(&impact.CompletedCount)
	q.Where("status = ?", models.MaintenanceStatusPlanned).Count(&impact.PlannedCount)
	q.Where("status = ?", models.MaintenanceStatusInProgress).Count(&impact.InProgressCount)
	q.Where("status = ?", models.MaintenanceStatusCancelled).Count(&impact.CancelledCount)
	q.Count(&impact.TotalTasks)

	if impact.TotalTasks > 0 {
		impact.CompletionRate = float64(impact.CompletedCount) / float64(impact.TotalTasks) * 100
	}

	// 计算损失时段和营收：仅统计未取消的维护
	var tasks []models.MaintenanceTask
	q2 := s.DB.Model(&models.MaintenanceTask{})
	if venueID != nil && *venueID > 0 {
		q2 = q2.Where("venue_id = ?", *venueID)
	}
	if startDate != "" {
		q2 = q2.Where("maintain_date >= ?", startDate)
	}
	if endDate != "" {
		q2 = q2.Where("maintain_date <= ?", endDate)
	}
	q2.Where("status <> ?", models.MaintenanceStatusCancelled).Find(&tasks)

	venuePriceCache := make(map[uint]float64)
	for _, t := range tasks {
		impact.ClosedSlots += t.DurationHours
		impact.TotalMaintenanceHours += t.DurationHours
		price, ok := venuePriceCache[t.VenueID]
		if !ok {
			var v models.Venue
			if err := s.DB.Select("hourly_price").First(&v, t.VenueID).Error; err == nil {
				price = v.HourlyPrice
				venuePriceCache[t.VenueID] = price
			}
		}
		impact.EstimatedRevenueLoss += price * float64(t.DurationHours)
	}

	// 平均耗时
	var completedTasks []models.MaintenanceTask
	s.DB.Model(&models.MaintenanceTask{}).
		Where("status = ? AND actual_duration IS NOT NULL", models.MaintenanceStatusCompleted).
		Find(&completedTasks)
	if len(completedTasks) > 0 {
		var total int
		for _, t := range completedTasks {
			if t.ActualDuration != nil {
				total += *t.ActualDuration
			}
		}
		impact.AvgDurationMinutes = float64(total) / float64(len(completedTasks))
	}

	return &impact, nil
}

// ============================================================
// 工具函数
// ============================================================

func parseWeekdays(s string) []int {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var res []int
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if v, err := strconv.Atoi(p); err == nil && v >= 1 && v <= 7 {
			res = append(res, v)
		}
	}
	return res
}

func makeDate(year, month, day int) time.Time {
	// 检查月末溢出
	t := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	lastDay := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.Local).Day()
	if day > lastDay {
		day = lastDay
	}
	return t.AddDate(0, 0, day-1)
}

func classifyConflict(bStart, bEnd, mStart, mEnd int) string {
	if bStart >= mStart && bEnd <= mEnd {
		return "full" // 预订完全被维护覆盖
	}
	return "overlap"
}

func (s *MaintenanceService) getVenue(id uint) (*models.Venue, error) {
	var v models.Venue
	err := s.DB.First(&v, id).Error
	return &v, err
}

// buildCandidateHours 生成候选时段：优先相同时段，然后向前向后找。
func buildCandidateHours(origStart, origEnd, openHour, closeHour int) [][2]int {
	duration := origEnd - origStart
	var result [][2]int
	// 1. 原时段
	result = append(result, [2]int{origStart, origEnd})
	// 2. 逐小时偏移
	for offset := 1; offset <= 24; offset++ {
		// 往前
		s := origStart - offset
		e := s + duration
		if s >= openHour && e <= closeHour {
			result = append(result, [2]int{s, e})
		}
		// 往后
		s = origStart + offset
		e = s + duration
		if s >= openHour && e <= closeHour {
			result = append(result, [2]int{s, e})
		}
	}
	return result
}
