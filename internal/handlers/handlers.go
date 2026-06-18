package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"venue-booking-admin/internal/auth"
	"venue-booking-admin/internal/models"
	"venue-booking-admin/internal/services"
)

// Handler 持有数据库句柄与服务。
type Handler struct {
	DB                *gorm.DB
	MaintenanceSvc    *services.MaintenanceService
}

// NewHandler 构造。
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{
		DB:             db,
		MaintenanceSvc: services.NewMaintenanceService(db),
	}
}

// ---------- 认证 ----------

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	var user models.User
	if err := h.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "用户名或密码错误"})
		return
	}
	if !auth.VerifyPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "用户名或密码错误"})
		return
	}
	token, err := auth.CreateToken(user.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "签发令牌失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"access_token": token, "token_type": "bearer"})
}

func (h *Handler) Me(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	c.JSON(http.StatusOK, gin.H{"id": user.ID, "username": user.Username, "display_name": user.DisplayName})
}

// ---------- 场馆 ----------

type venueReq struct {
	Name        string  `json:"name" binding:"required"`
	SportType   string  `json:"sport_type"`
	Capacity    int     `json:"capacity"`
	HourlyPrice float64 `json:"hourly_price"`
	OpenHour    int     `json:"open_hour"`
	CloseHour   int     `json:"close_hour"`
	Status      string  `json:"status"`
}

func (h *Handler) ListVenues(c *gin.Context) {
	var venues []models.Venue
	q := h.DB.Order("id")
	if st := c.Query("sport_type"); st != "" {
		q = q.Where("sport_type = ?", st)
	}
	q.Find(&venues)
	c.JSON(http.StatusOK, venues)
}

func (h *Handler) CreateVenue(c *gin.Context) {
	var req venueReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	if req.CloseHour <= req.OpenHour {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "关闭时间须晚于开放时间"})
		return
	}
	status := req.Status
	if status == "" {
		status = "open"
	}
	venue := models.Venue{
		Name: req.Name, SportType: req.SportType, Capacity: req.Capacity,
		HourlyPrice: req.HourlyPrice, OpenHour: req.OpenHour, CloseHour: req.CloseHour, Status: status,
	}
	h.DB.Create(&venue)
	c.JSON(http.StatusCreated, venue)
}

func (h *Handler) GetVenue(c *gin.Context) {
	var venue models.Venue
	if err := h.DB.First(&venue, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "场馆不存在"})
		return
	}
	c.JSON(http.StatusOK, venue)
}

func (h *Handler) UpdateVenue(c *gin.Context) {
	var venue models.Venue
	if err := h.DB.First(&venue, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "场馆不存在"})
		return
	}
	var req venueReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	venue.Name = req.Name
	venue.SportType = req.SportType
	venue.Capacity = req.Capacity
	venue.HourlyPrice = req.HourlyPrice
	if req.OpenHour != 0 || req.CloseHour != 0 {
		venue.OpenHour = req.OpenHour
		venue.CloseHour = req.CloseHour
	}
	if req.Status != "" {
		venue.Status = req.Status
	}
	h.DB.Save(&venue)
	c.JSON(http.StatusOK, venue)
}

func (h *Handler) DeleteVenue(c *gin.Context) {
	var venue models.Venue
	if err := h.DB.First(&venue, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "场馆不存在"})
		return
	}
	h.DB.Delete(&venue)
	c.Status(http.StatusNoContent)
}

// ---------- 预订 ----------

type bookingReq struct {
	VenueID      uint   `json:"venue_id" binding:"required"`
	CustomerName string `json:"customer_name" binding:"required"`
	Phone        string `json:"phone"`
	BookDate     string `json:"book_date" binding:"required"`
	StartHour    int    `json:"start_hour"`
	EndHour      int    `json:"end_hour"`
}

func (h *Handler) ListBookings(c *gin.Context) {
	var bookings []models.Booking
	q := h.DB.Order("id desc")
	if vid := c.Query("venue_id"); vid != "" {
		q = q.Where("venue_id = ?", vid)
	}
	if d := c.Query("date"); d != "" {
		q = q.Where("book_date = ?", d)
	}
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	q.Find(&bookings)
	c.JSON(http.StatusOK, bookings)
}

func (h *Handler) CreateBooking(c *gin.Context) {
	var req bookingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	if req.EndHour <= req.StartHour {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "结束时间须晚于开始时间"})
		return
	}
	var venue models.Venue
	if err := h.DB.First(&venue, req.VenueID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "场馆不存在"})
		return
	}
	if venue.Status != "open" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "该场馆当前不可预订"})
		return
	}
	if req.StartHour < venue.OpenHour || req.EndHour > venue.CloseHour {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "预订时段超出场馆开放时间"})
		return
	}

	// 时段冲突校验：同场馆同日，已有非取消预订时段不得与本次重叠
	var conflict int64
	h.DB.Model(&models.Booking{}).
		Where("venue_id = ? AND book_date = ? AND status NOT IN ?", req.VenueID, req.BookDate, []string{"cancelled", "rescheduled"}).
		Where("start_hour < ? AND end_hour > ?", req.EndHour, req.StartHour).
		Count(&conflict)
	if conflict > 0 {
		c.JSON(http.StatusConflict, gin.H{"detail": "该时段已被预订"})
		return
	}

	// 【新增】维护时段冲突校验
	maintenanceConflicts, err := h.MaintenanceSvc.CheckBookingVsMaintenance(req.VenueID, req.BookDate, req.StartHour, req.EndHour)
	if err == nil && len(maintenanceConflicts) > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"detail": "该时段已安排维护，无法预订",
			"maintenance_conflicts": maintenanceConflicts,
		})
		return
	}

	amount := venue.HourlyPrice * float64(req.EndHour-req.StartHour)
	booking := models.Booking{
		VenueID: req.VenueID, CustomerName: req.CustomerName, Phone: req.Phone,
		BookDate: req.BookDate, StartHour: req.StartHour, EndHour: req.EndHour,
		Amount: amount, Status: "booked",
	}
	h.DB.Create(&booking)
	c.JSON(http.StatusCreated, booking)
}

type statusReq struct {
	Status string `json:"status" binding:"required"`
}

func (h *Handler) UpdateBookingStatus(c *gin.Context) {
	var booking models.Booking
	if err := h.DB.First(&booking, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "预订不存在"})
		return
	}
	var req statusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "状态不合法"})
		return
	}
	allowed := map[string]bool{"booked": true, "cancelled": true, "completed": true, "rescheduled": true}
	if !allowed[req.Status] {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "状态不合法"})
		return
	}
	booking.Status = req.Status
	h.DB.Save(&booking)
	c.JSON(http.StatusOK, gin.H{"id": booking.ID, "status": booking.Status})
}

// ---------- 仪表盘 ----------

func (h *Handler) DashboardStats(c *gin.Context) {
	var venueTotal, venueOpen, bookingTotal, bookingActive int64
	h.DB.Model(&models.Venue{}).Count(&venueTotal)
	h.DB.Model(&models.Venue{}).Where("status = ?", "open").Count(&venueOpen)
	h.DB.Model(&models.Booking{}).Count(&bookingTotal)
	h.DB.Model(&models.Booking{}).Where("status = ?", "booked").Count(&bookingActive)

	var revenue float64
	h.DB.Model(&models.Booking{}).Where("status <> ?", "cancelled").
		Select("COALESCE(SUM(amount),0)").Scan(&revenue)

	c.JSON(http.StatusOK, gin.H{
		"venue_total":     venueTotal,
		"venue_open":      venueOpen,
		"booking_total":   bookingTotal,
		"booking_active":  bookingActive,
		"revenue_total":   revenue,
	})
}

// Health 健康检查。
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "venue-booking-admin"})
}

// ============================================================
// 维护任务 CRUD
// ============================================================

type maintenanceTaskReq struct {
	VenueID     uint                       `json:"venue_id" binding:"required"`
	Title       string                     `json:"title" binding:"required"`
	Type        models.MaintenanceType     `json:"type"`
	Priority    models.MaintenancePriority `json:"priority"`
	Status      models.MaintenanceStatus   `json:"status"`
	Assignee    string                     `json:"assignee"`
	Description string                     `json:"description"`

	MaintainDate  string `json:"maintain_date" binding:"required"`
	StartHour     int    `json:"start_hour" binding:"required"`
	EndHour       int    `json:"end_hour" binding:"required"`
}

func (h *Handler) CreateMaintenanceTask(c *gin.Context) {
	var req maintenanceTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法: " + err.Error()})
		return
	}
	if req.EndHour <= req.StartHour {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "结束小时必须晚于开始小时"})
		return
	}
	if req.Type == "" {
		req.Type = models.MaintenanceTypeRoutine
	}
	if req.Priority == "" {
		req.Priority = models.MaintenancePriorityMedium
	}

	task := &models.MaintenanceTask{
		VenueID:       req.VenueID,
		Title:         req.Title,
		Type:          req.Type,
		Priority:      req.Priority,
		Status:        req.Status,
		Assignee:      req.Assignee,
		Description:   req.Description,
		MaintainDate:  req.MaintainDate,
		StartHour:     req.StartHour,
		EndHour:       req.EndHour,
		DurationHours: req.EndHour - req.StartHour,
		IsPeriodic:    false,
	}

	result, err := h.MaintenanceSvc.CreateMaintenanceTaskWithConflictCheck(task)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, result)
}

func (h *Handler) ListMaintenanceTasks(c *gin.Context) {
	var tasks []models.MaintenanceTask
	q := h.DB.Order("maintain_date desc, start_hour")
	if vid := c.Query("venue_id"); vid != "" {
		if v, e := strconv.Atoi(vid); e == nil {
			q = q.Where("venue_id = ?", v)
		}
	}
	if sd := c.Query("start_date"); sd != "" {
		q = q.Where("maintain_date >= ?", sd)
	}
	if ed := c.Query("end_date"); ed != "" {
		q = q.Where("maintain_date <= ?", ed)
	}
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	if tp := c.Query("type"); tp != "" {
		q = q.Where("type = ?", tp)
	}
	q.Find(&tasks)
	c.JSON(http.StatusOK, tasks)
}

func (h *Handler) GetMaintenanceTask(c *gin.Context) {
	var task models.MaintenanceTask
	if err := h.DB.First(&task, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "维护任务不存在"})
		return
	}

	var conflicts []models.MaintenanceConflict
	h.DB.Where("maintenance_task_id = ?", task.ID).Find(&conflicts)

	c.JSON(http.StatusOK, gin.H{
		"task":      task,
		"conflicts": conflicts,
	})
}

func (h *Handler) UpdateMaintenanceTask(c *gin.Context) {
	var task models.MaintenanceTask
	if err := h.DB.First(&task, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "维护任务不存在"})
		return
	}
	var req maintenanceTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	if req.EndHour <= req.StartHour {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "结束小时必须晚于开始小时"})
		return
	}
	task.VenueID = req.VenueID
	task.Title = req.Title
	task.Type = req.Type
	task.Priority = req.Priority
	task.Assignee = req.Assignee
	task.Description = req.Description
	task.MaintainDate = req.MaintainDate
	task.StartHour = req.StartHour
	task.EndHour = req.EndHour
	task.DurationHours = req.EndHour - req.StartHour
	h.DB.Save(&task)
	c.JSON(http.StatusOK, task)
}

func (h *Handler) DeleteMaintenanceTask(c *gin.Context) {
	var task models.MaintenanceTask
	if err := h.DB.First(&task, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "维护任务不存在"})
		return
	}
	// 软删除：改为 cancelled 状态，保留历史记录
	task.Status = models.MaintenanceStatusCancelled
	h.DB.Save(&task)
	c.JSON(http.StatusOK, gin.H{"id": task.ID, "status": task.Status})
}

// ============================================================
// 维护任务状态流转 / 执行记录
// ============================================================

type maintenanceStatusReq struct {
	Status       models.MaintenanceStatus `json:"status" binding:"required"`
	ExecutorNote string                   `json:"executor_note"`
	ExecutorName string                   `json:"executor_name"`
	ReplacedParts string                  `json:"replaced_parts"`
}

func (h *Handler) UpdateMaintenanceStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "ID不合法"})
		return
	}
	var req maintenanceStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	task, err := h.MaintenanceSvc.UpdateMaintenanceStatus(uint(id), req.Status, req.ExecutorNote, req.ExecutorName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	if req.ReplacedParts != "" {
		task.ReplacedParts = req.ReplacedParts
		h.DB.Save(task)
	}
	c.JSON(http.StatusOK, task)
}

// ============================================================
// 周期维护规则
// ============================================================

type maintenanceRuleReq struct {
	VenueID   uint                       `json:"venue_id" binding:"required"`
	Title     string                     `json:"title" binding:"required"`
	Type      models.MaintenanceType     `json:"type"`
	Priority  models.MaintenancePriority `json:"priority"`
	Assignee  string                     `json:"assignee"`
	Description string                   `json:"description"`

	RecurrenceType     string `json:"recurrence_type" binding:"required"` // daily/weekly/monthly/yearly
	RecurrenceInterval int    `json:"recurrence_interval"`
	Weekdays           string `json:"weekdays"`
	MonthDay           int    `json:"month_day"`

	StartHour int `json:"start_hour" binding:"required"`
	EndHour   int `json:"end_hour" binding:"required"`

	StartDate string `json:"start_date" binding:"required"`
	EndDate   string `json:"end_date" binding:"required"`
}

func (h *Handler) CreateMaintenanceRule(c *gin.Context) {
	var req maintenanceRuleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法: " + err.Error()})
		return
	}
	if req.EndHour <= req.StartHour {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "结束小时必须晚于开始小时"})
		return
	}
	if req.Type == "" {
		req.Type = models.MaintenanceTypeRoutine
	}
	if req.Priority == "" {
		req.Priority = models.MaintenancePriorityMedium
	}
	rule := &models.MaintenanceRule{
		VenueID:            req.VenueID,
		Title:              req.Title,
		Type:               req.Type,
		Priority:           req.Priority,
		Assignee:           req.Assignee,
		Description:        req.Description,
		RecurrenceType:     req.RecurrenceType,
		RecurrenceInterval: req.RecurrenceInterval,
		Weekdays:           req.Weekdays,
		MonthDay:           req.MonthDay,
		StartHour:          req.StartHour,
		EndHour:            req.EndHour,
		DurationHours:      req.EndHour - req.StartHour,
		StartDate:          req.StartDate,
		EndDate:            req.EndDate,
	}
	r, tasks, err := h.MaintenanceSvc.CreateMaintenanceRuleAndExpand(rule)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"rule":          r,
		"expanded_count": len(tasks),
		"expanded_tasks": tasks,
	})
}

func (h *Handler) ListMaintenanceRules(c *gin.Context) {
	var rules []models.MaintenanceRule
	q := h.DB.Order("id desc")
	if vid := c.Query("venue_id"); vid != "" {
		if v, e := strconv.Atoi(vid); e == nil {
			q = q.Where("venue_id = ?", v)
		}
	}
	if st := c.Query("status"); st != "" {
		q = q.Where("status = ?", st)
	}
	q.Find(&rules)
	c.JSON(http.StatusOK, rules)
}

func (h *Handler) GetMaintenanceRule(c *gin.Context) {
	var rule models.MaintenanceRule
	if err := h.DB.First(&rule, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "规则不存在"})
		return
	}
	// 预览展开
	tasks, err := h.MaintenanceSvc.ExpandMaintenanceRule(&rule)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"rule": rule, "expand_error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rule": rule, "preview_expand_count": len(tasks), "preview_tasks": tasks})
}

func (h *Handler) ToggleMaintenanceRule(c *gin.Context) {
	var rule models.MaintenanceRule
	if err := h.DB.First(&rule, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "规则不存在"})
		return
	}
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	rule.Status = req.Status
	h.DB.Save(&rule)
	c.JSON(http.StatusOK, rule)
}

// ============================================================
// 冲突检测 & 改约建议
// ============================================================

type conflictCheckReq struct {
	VenueID      uint   `json:"venue_id" binding:"required"`
	MaintainDate string `json:"maintain_date" binding:"required"`
	StartHour    int    `json:"start_hour" binding:"required"`
	EndHour      int    `json:"end_hour" binding:"required"`
}

func (h *Handler) CheckMaintenanceConflicts(c *gin.Context) {
	var req conflictCheckReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	if req.EndHour <= req.StartHour {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "结束小时必须晚于开始小时"})
		return
	}
	bookings, err := h.MaintenanceSvc.CheckMaintenanceVsBookings(req.VenueID, req.MaintainDate, req.StartHour, req.EndHour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	suggestionsMap := make(map[uint][]models.RescheduleSuggestion)
	for _, b := range bookings {
		sugs, _ := h.MaintenanceSvc.GenerateRescheduleSuggestions(b, 30)
		suggestionsMap[b.ID] = sugs
	}

	c.JSON(http.StatusOK, gin.H{
		"has_conflict":           len(bookings) > 0,
		"conflict_count":         len(bookings),
		"conflict_bookings":      bookings,
		"suggestions_by_booking": suggestionsMap,
	})
}

type bookingSuggestReq struct {
	BookingID uint `json:"booking_id" binding:"required"`
	MaxDays   int  `json:"max_days"`
}

func (h *Handler) GetRescheduleSuggestions(c *gin.Context) {
	var req bookingSuggestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	if req.MaxDays <= 0 {
		req.MaxDays = 30
	}
	var booking models.Booking
	if err := h.DB.First(&booking, req.BookingID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "预订不存在"})
		return
	}
	sugs, err := h.MaintenanceSvc.GenerateRescheduleSuggestions(booking, req.MaxDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"booking": booking, "suggestions": sugs, "count": len(sugs)})
}

// ============================================================
// 冲突解决：改约 / 退订
// ============================================================

type resolveSingleReq struct {
	TaskID     uint                           `json:"task_id" binding:"required"`
	BookingID  uint                           `json:"booking_id" binding:"required"`
	Action     string                         `json:"action" binding:"required"` // reschedule / refund
	Suggestion *models.RescheduleSuggestion   `json:"suggestion,omitempty"`
}

func (h *Handler) ResolveSingleConflict(c *gin.Context) {
	var req resolveSingleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法: " + err.Error()})
		return
	}
	if req.Action == "reschedule" && req.Suggestion == nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "改约必须指定suggestion"})
		return
	}
	var sug *models.RescheduleSuggestion
	if req.Action == "reschedule" {
		sug = req.Suggestion
	}
	newBooking, err := h.MaintenanceSvc.ResolveSingleConflict(req.TaskID, req.BookingID, sug)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	resp := gin.H{
		"task_id":    req.TaskID,
		"booking_id": req.BookingID,
		"action":     req.Action,
	}
	if newBooking != nil {
		resp["new_booking"] = newBooking
	}
	c.JSON(http.StatusOK, resp)
}

type batchResolveReq struct {
	TaskID       uint   `json:"task_id" binding:"required"`
	Strategy     string `json:"strategy"` // auto / refund
}

func (h *Handler) BatchResolveConflicts(c *gin.Context) {
	var req batchResolveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数不合法"})
		return
	}
	strategy := services.ResolveStrategy{Auto: true}
	if req.Strategy == "refund" {
		strategy = services.ResolveStrategy{ForceRefund: true}
	}
	result, err := h.MaintenanceSvc.BatchResolveConflicts(req.TaskID, strategy)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// ============================================================
// 维护日历 & 空档
// ============================================================

func (h *Handler) MaintenanceCalendar(c *gin.Context) {
	var venueID *uint
	if vid := c.Query("venue_id"); vid != "" {
		if v, e := strconv.Atoi(vid); e == nil && v > 0 {
			uv := uint(v)
			venueID = &uv
		}
	}
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	var statuses []string
	if st := c.Query("status"); st != "" {
		statuses = strings.Split(st, ",")
	}
	tasks, err := h.MaintenanceSvc.GetMaintenanceCalendar(venueID, startDate, endDate, statuses)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tasks)
}

func (h *Handler) VenueFreeSlots(c *gin.Context) {
	vid := c.Query("venue_id")
	if vid == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "venue_id参数必填"})
		return
	}
	venueID, err := strconv.ParseUint(vid, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "venue_id不合法"})
		return
	}
	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "date参数必填"})
		return
	}
	slots, err := h.MaintenanceSvc.GetVenueFreeSlots(uint(venueID), dateStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"venue_id": venueID,
		"date":     dateStr,
		"slots":    slots,
	})
}

// ============================================================
// 统计分析
// ============================================================

func (h *Handler) MaintenanceStats(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	var venueID *uint
	if vid := c.Query("venue_id"); vid != "" {
		if v, e := strconv.Atoi(vid); e == nil && v > 0 {
			uv := uint(v)
			venueID = &uv
		}
	}
	impact, err := h.MaintenanceSvc.GetMaintenanceStats(startDate, endDate, venueID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, impact)
}

// ============================================================
// 冲突记录列表（审计）
// ============================================================

func (h *Handler) ListMaintenanceConflicts(c *gin.Context) {
	var conflicts []models.MaintenanceConflict
	q := h.DB.Order("id desc")
	if tid := c.Query("task_id"); tid != "" {
		if v, e := strconv.Atoi(tid); e == nil {
			q = q.Where("maintenance_task_id = ?", v)
		}
	}
	if bid := c.Query("booking_id"); bid != "" {
		if v, e := strconv.Atoi(bid); e == nil {
			q = q.Where("booking_id = ?", v)
		}
	}
	if rt := c.Query("resolution_type"); rt != "" {
		q = q.Where("resolution_type = ?", rt)
	}
	q.Find(&conflicts)
	c.JSON(http.StatusOK, conflicts)
}
