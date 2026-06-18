package seed

import (
	"log"
	"time"

	"gorm.io/gorm"

	"venue-booking-admin/internal/auth"
	"venue-booking-admin/internal/models"
)

// Run 初始化内置管理员与种子业务数据（幂等）。
func Run(database *gorm.DB, adminUser, adminPass string) error {
	var count int64
	database.Model(&models.User{}).Where("username = ?", adminUser).Count(&count)
	if count == 0 {
		hash, err := auth.HashPassword(adminPass)
		if err != nil {
			return err
		}
		database.Create(&models.User{Username: adminUser, PasswordHash: hash, DisplayName: "平台管理员"})
		log.Println("已创建管理员账号")
	}

	var venueCount int64
	database.Model(&models.Venue{}).Count(&venueCount)
	if venueCount > 0 {
		// 已存在数据，仅补充维护相关种子数据
		return seedMaintenanceData(database)
	}

	venues := []models.Venue{
		{Name: "城北全民健身中心篮球馆A", SportType: "basketball", Capacity: 200, HourlyPrice: 160, OpenHour: 8, CloseHour: 22, Status: "open"},
		{Name: "城北全民健身中心篮球馆B", SportType: "basketball", Capacity: 150, HourlyPrice: 140, OpenHour: 8, CloseHour: 22, Status: "open"},
		{Name: "奥体中心游泳馆", SportType: "swimming", Capacity: 400, HourlyPrice: 80, OpenHour: 6, CloseHour: 21, Status: "open"},
		{Name: "市民广场羽毛球馆1号", SportType: "badminton", Capacity: 60, HourlyPrice: 50, OpenHour: 9, CloseHour: 22, Status: "open"},
		{Name: "市民广场羽毛球馆2号", SportType: "badminton", Capacity: 50, HourlyPrice: 45, OpenHour: 9, CloseHour: 22, Status: "open"},
		{Name: "滨江足球公园主场", SportType: "football", Capacity: 500, HourlyPrice: 300, OpenHour: 8, CloseHour: 20, Status: "open"},
		{Name: "滨江足球公园训练场", SportType: "football", Capacity: 200, HourlyPrice: 200, OpenHour: 8, CloseHour: 22, Status: "open"},
		{Name: "奥体中心网球场", SportType: "tennis", Capacity: 80, HourlyPrice: 120, OpenHour: 7, CloseHour: 22, Status: "maintenance"},
	}
	if err := database.Create(&venues).Error; err != nil {
		return err
	}
	log.Printf("已创建 %d 个场馆种子数据", len(venues))

	bookings := []models.Booking{
		// 篮球馆A - 6月20日
		{VenueID: venues[0].ID, CustomerName: "陈刚", Phone: "13700001111", BookDate: "2026-06-20", StartHour: 18, EndHour: 20, Amount: 320, Status: "booked"},
		{VenueID: venues[0].ID, CustomerName: "周敏", Phone: "13700002222", BookDate: "2026-06-20", StartHour: 20, EndHour: 21, Amount: 160, Status: "booked"},
		{VenueID: venues[0].ID, CustomerName: "赵磊", Phone: "13700005555", BookDate: "2026-06-20", StartHour: 15, EndHour: 17, Amount: 320, Status: "booked"},
		{VenueID: venues[0].ID, CustomerName: "孙悦", Phone: "13700006666", BookDate: "2026-06-21", StartHour: 9, EndHour: 11, Amount: 320, Status: "booked"},
		{VenueID: venues[0].ID, CustomerName: "李强", Phone: "13700007777", BookDate: "2026-06-22", StartHour: 19, EndHour: 21, Amount: 320, Status: "booked"},

		// 篮球馆B
		{VenueID: venues[1].ID, CustomerName: "王伟", Phone: "13700008888", BookDate: "2026-06-20", StartHour: 18, EndHour: 20, Amount: 280, Status: "booked"},
		{VenueID: venues[1].ID, CustomerName: "刘洋", Phone: "13700009999", BookDate: "2026-06-21", StartHour: 14, EndHour: 16, Amount: 280, Status: "booked"},

		// 游泳馆
		{VenueID: venues[2].ID, CustomerName: "黄磊", Phone: "13700003333", BookDate: "2026-06-21", StartHour: 7, EndHour: 9, Amount: 160, Status: "completed"},
		{VenueID: venues[2].ID, CustomerName: "郑洁", Phone: "13700011111", BookDate: "2026-06-22", StartHour: 8, EndHour: 10, Amount: 160, Status: "booked"},
		{VenueID: venues[2].ID, CustomerName: "马超", Phone: "13700012222", BookDate: "2026-06-23", StartHour: 15, EndHour: 17, Amount: 160, Status: "booked"},

		// 羽毛球馆1号
		{VenueID: venues[3].ID, CustomerName: "林燕", Phone: "13700013333", BookDate: "2026-06-21", StartHour: 19, EndHour: 21, Amount: 100, Status: "booked"},
		{VenueID: venues[3].ID, CustomerName: "何勇", Phone: "13700014444", BookDate: "2026-06-25", StartHour: 10, EndHour: 12, Amount: 100, Status: "booked"},

		// 足球公园
		{VenueID: venues[5].ID, CustomerName: "吴静", Phone: "13700004444", BookDate: "2026-06-22", StartHour: 15, EndHour: 17, Amount: 600, Status: "cancelled"},
		{VenueID: venues[5].ID, CustomerName: "杨帆", Phone: "13700015555", BookDate: "2026-06-28", StartHour: 9, EndHour: 12, Amount: 900, Status: "booked"},

		// 足球训练场 - 与维护计划冲突的预订（用于测试冲突场景）
		{VenueID: venues[6].ID, CustomerName: "朱军", Phone: "13700016666", BookDate: "2026-06-25", StartHour: 8, EndHour: 10, Amount: 400, Status: "booked"},
		{VenueID: venues[6].ID, CustomerName: "徐颖", Phone: "13700017777", BookDate: "2026-06-25", StartHour: 10, EndHour: 12, Amount: 400, Status: "booked"},
		{VenueID: venues[6].ID, CustomerName: "胡斌", Phone: "13700018888", BookDate: "2026-07-02", StartHour: 9, EndHour: 11, Amount: 400, Status: "booked"},
	}
	if err := database.Create(&bookings).Error; err != nil {
		return err
	}
	log.Printf("已创建 %d 条预订种子数据", len(bookings))

	return seedMaintenanceData(database)
}

func seedMaintenanceData(database *gorm.DB) error {
	var taskCount int64
	database.Model(&models.MaintenanceTask{}).Count(&taskCount)
	if taskCount > 0 {
		return nil
	}

	// 查询场馆ID用于关联
	var venues []models.Venue
	database.Order("id").Find(&venues)
	if len(venues) < 6 {
		log.Println("场馆数量不足，跳过维护种子数据")
		return nil
	}

	// ===== 周期维护规则 =====
	rules := []models.MaintenanceRule{
		{
			VenueID:            venues[2].ID, // 游泳馆
			Title:              "游泳馆每月水质检测与设备保养",
			Type:               models.MaintenanceTypeRoutine,
			Priority:           models.MaintenancePriorityMedium,
			Assignee:           "李师傅",
			Description:        "每月固定日执行水质检测、过滤系统清洁、消毒剂补充",
			RecurrenceType:     "monthly",
			RecurrenceInterval: 1,
			MonthDay:           15,
			StartHour:          6,
			EndHour:            9,
			DurationHours:      3,
			StartDate:          "2026-06-01",
			EndDate:            "2026-12-31",
			Status:             "active",
		},
		{
			VenueID:            venues[3].ID, // 羽毛球馆1号
			Title:              "羽毛球馆每周一网前维护",
			Type:               models.MaintenanceTypeRoutine,
			Priority:           models.MaintenancePriorityLow,
			Assignee:           "王师傅",
			Description:        "每周一早上球网拉紧、地胶清洁、灯光检查",
			RecurrenceType:     "weekly",
			RecurrenceInterval: 1,
			Weekdays:           "1",
			StartHour:          7,
			EndHour:            9,
			DurationHours:      2,
			StartDate:          "2026-06-01",
			EndDate:            "2026-12-31",
			Status:             "active",
		},
		{
			VenueID:            venues[5].ID, // 足球公园主场
			Title:              "足球场每两周草坪修剪与划线",
			Type:               models.MaintenanceTypeRoutine,
			Priority:           models.MaintenancePriorityMedium,
			Assignee:           "赵师傅",
			Description:        "每两周进行草坪修剪、场地划线、球门检查",
			RecurrenceType:     "weekly",
			RecurrenceInterval: 2,
			Weekdays:           "3",
			StartHour:          8,
			EndHour:            12,
			DurationHours:      4,
			StartDate:          "2026-06-01",
			EndDate:            "2026-12-31",
			Status:             "active",
		},
	}

	maintSvc := newSeedMaintenanceService(database)
	var createdTaskCount int
	for _, r := range rules {
		rule := r
		_, tasks, err := maintSvc.CreateMaintenanceRuleAndExpand(&rule)
		if err == nil {
			createdTaskCount += len(tasks)
		}
	}
	log.Printf("已创建 %d 条周期维护规则，展开生成 %d 次维护任务", len(rules), createdTaskCount)

	// ===== 单次维护任务 =====
	now := timeNow()
	tasks := []models.MaintenanceTask{
		// 已完成：篮球馆A 6月10日地板打蜡
		{
			VenueID: venues[0].ID, Title: "篮球馆A地板打蜡抛光",
			Type: models.MaintenanceTypeRoutine, Priority: models.MaintenancePriorityLow,
			Status: models.MaintenanceStatusCompleted, Assignee: "张师傅",
			Description: "赛前场地维护，地板打蜡防滑处理",
			MaintainDate: "2026-06-10", StartHour: 14, EndHour: 17, DurationHours: 3,
			IsPeriodic: false, ActualStartAt: &now, ActualEndAt: &now,
			ActualDuration: intPtr(165), ResultNote: "打蜡完成，防滑测试合格",
			ExecutorName: "张师傅", CompletedAt: &now,
		},
		// 进行中：网球场设施升级（status=maintenance的那个场馆）
		{
			VenueID: venues[7].ID, Title: "网球场地面材质升级改造",
			Type: models.MaintenanceTypeUpgrade, Priority: models.MaintenancePriorityHigh,
			Status: models.MaintenanceStatusInProgress, Assignee: "工程队A组",
			Description: "更换塑胶面层，升级为丙烯酸材质",
			MaintainDate: "2026-06-18", StartHour: 8, EndHour: 18, DurationHours: 10,
			IsPeriodic: false, ActualStartAt: &now, ExecutorName: "工程队A组",
		},
		// 计划中：足球训练场6月25日 - 与已有预订冲突（8-12点，预订是8-10和10-12）
		{
			VenueID: venues[6].ID, Title: "足球训练场草坪紧急修补",
			Type: models.MaintenanceTypeRepair, Priority: models.MaintenancePriorityHigh,
			Status: models.MaintenanceStatusPlanned, Assignee: "赵师傅",
			Description: "中场区域草皮严重磨损，需更换草皮块并填充",
			MaintainDate: "2026-06-25", StartHour: 8, EndHour: 12, DurationHours: 4,
			IsPeriodic: false,
		},
		// 计划中：篮球馆B 6月24日日常保养
		{
			VenueID: venues[1].ID, Title: "篮球馆B篮板检查与篮筐校准",
			Type: models.MaintenanceTypeRoutine, Priority: models.MaintenancePriorityMedium,
			Status: models.MaintenanceStatusPlanned, Assignee: "张师傅",
			Description: "检查篮板稳固性，校准篮筐高度与水平",
			MaintainDate: "2026-06-24", StartHour: 10, EndHour: 12, DurationHours: 2,
			IsPeriodic: false,
		},
		// 已取消：羽毛球馆2号的维护计划已取消
		{
			VenueID: venues[4].ID, Title: "羽毛球馆2号地胶更换",
			Type: models.MaintenanceTypeUpgrade, Priority: models.MaintenancePriorityMedium,
			Status: models.MaintenanceStatusCancelled, Assignee: "王师傅",
			Description: "原计划更换地胶，因物料未到而取消",
			MaintainDate: "2026-06-17", StartHour: 9, EndHour: 12, DurationHours: 3,
			IsPeriodic: false,
		},
		// 计划中：游泳馆6月20日过滤器清洁（额外的临时维修）
		{
			VenueID: venues[2].ID, Title: "游泳馆循环过滤器临时维修",
			Type: models.MaintenanceTypeRepair, Priority: models.MaintenancePriorityUrgent,
			Status: models.MaintenanceStatusPlanned, Assignee: "李师傅",
			Description: "过滤系统压力异常，需紧急检修",
			MaintainDate: "2026-06-20", StartHour: 12, EndHour: 15, DurationHours: 3,
			IsPeriodic: false,
		},
	}

	for i := range tasks {
		t := tasks[i]
		if err := database.Create(&t).Error; err == nil {
			// 如果是6月25日的足球训练场维护，自动生成冲突记录（因为有预订）
			if t.MaintainDate == "2026-06-25" && t.VenueID == venues[6].ID {
				var conflictBookings []models.Booking
				database.Where("venue_id = ? AND book_date = ? AND status NOT IN ?",
					venues[6].ID, "2026-06-25", []string{"cancelled", "completed", "rescheduled"}).
					Where("start_hour < ? AND end_hour > ?", t.EndHour, t.StartHour).
					Find(&conflictBookings)
				for _, b := range conflictBookings {
					database.Create(&models.MaintenanceConflict{
						MaintenanceTaskID: t.ID,
						BookingID:         b.ID,
						ConflictType:      "full",
						ResolutionType:    "pending",
					})
				}
				log.Printf("为足球训练场6月25日维护生成 %d 条冲突记录", len(conflictBookings))
			}
		}
	}
	log.Printf("已创建 %d 条单次维护任务种子数据", len(tasks))
	return nil
}

// ===== 种子辅助函数（简化版，不依赖完整services包以避免循环） =====
type seedMaintSvc struct{ DB *gorm.DB }

func newSeedMaintenanceService(db *gorm.DB) *seedMaintSvc { return &seedMaintSvc{DB: db} }

func (s *seedMaintSvc) CreateMaintenanceRuleAndExpand(rule *models.MaintenanceRule) (*models.MaintenanceRule, []models.MaintenanceTask, error) {
	rule.Status = "active"
	if err := s.DB.Create(rule).Error; err != nil {
		return nil, nil, err
	}
	tasks := expandSeedRule(rule)
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

func expandSeedRule(rule *models.MaintenanceRule) []models.MaintenanceTask {
	const df = "2006-01-02"
	start, _ := time.Parse(df, rule.StartDate)
	end, _ := time.Parse(df, rule.EndDate)
	var tasks []models.MaintenanceTask

	makeTask := func(d time.Time) models.MaintenanceTask {
		return models.MaintenanceTask{
			VenueID: rule.VenueID, Title: rule.Title, Type: rule.Type,
			Priority: rule.Priority, Status: models.MaintenanceStatusPlanned,
			Assignee: rule.Assignee, Description: rule.Description,
			MaintainDate: d.Format(df), StartHour: rule.StartHour,
			EndHour: rule.EndHour, DurationHours: rule.EndHour - rule.StartHour,
			IsPeriodic: true,
		}
	}

	interval := rule.RecurrenceInterval
	if interval <= 0 {
		interval = 1
	}

	switch rule.RecurrenceType {
	case "monthly":
		day := rule.MonthDay
		if day <= 0 {
			day = 15
		}
		for y, m := start.Year(), int(start.Month()); y <= end.Year(); {
			candidate := safeDate(y, m, day)
			if candidate.After(end) {
				break
			}
			if !candidate.Before(start) {
				tasks = append(tasks, makeTask(candidate))
			}
			m += interval
			for m > 12 {
				m -= 12
				y++
			}
		}
	case "weekly":
		wdTarget := 1 // 周一
		if rule.Weekdays != "" {
			wdTarget = int(rule.Weekdays[0] - '0')
		}
		// 找到第一个目标日
		wd := int(start.Weekday())
		if wd == 0 {
			wd = 7
		}
		first := start.AddDate(0, 0, (wdTarget-wd+7)%7)
		for d := first; !d.After(end); d = d.AddDate(0, 0, 7*interval) {
			tasks = append(tasks, makeTask(d))
		}
	}
	return tasks
}

func safeDate(year, month, day int) time.Time {
	t := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	last := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.Local).Day()
	if day > last {
		day = last
	}
	return t.AddDate(0, 0, day-1)
}

func intPtr(n int) *int        { return &n }
func timeNow() time.Time       { return time.Date(2026, 6, 18, 12, 0, 0, 0, time.Local) }
