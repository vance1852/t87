package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"venue-booking-admin/internal/auth"
	"venue-booking-admin/internal/config"
	"venue-booking-admin/internal/db"
	"venue-booking-admin/internal/handlers"
	"venue-booking-admin/internal/seed"
)

func main() {
	cfg := config.Load()
	auth.SetSecret(cfg.JWTSecret)

	database, err := db.Connect(cfg.DSN)
	if err != nil {
		log.Fatalf("无法连接数据库: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
	if err := seed.Run(database, cfg.AdminUsername, cfg.AdminPassword); err != nil {
		log.Fatalf("种子数据初始化失败: %v", err)
	}

	h := handlers.NewHandler(database)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	api := r.Group("/api")
	{
		api.GET("/health", h.Health)
		api.POST("/auth/login", h.Login)

		secured := api.Group("")
		secured.Use(auth.Middleware(database))
		{
			secured.GET("/auth/me", h.Me)

			secured.GET("/venues", h.ListVenues)
			secured.POST("/venues", h.CreateVenue)
			secured.GET("/venues/:id", h.GetVenue)
			secured.PUT("/venues/:id", h.UpdateVenue)
			secured.DELETE("/venues/:id", h.DeleteVenue)
			secured.GET("/venues/:venue_id/free-slots", h.VenueFreeSlots)

			secured.GET("/bookings", h.ListBookings)
			secured.POST("/bookings", h.CreateBooking)
			secured.PATCH("/bookings/:id/status", h.UpdateBookingStatus)

			secured.GET("/dashboard/stats", h.DashboardStats)

			// ====== 维护任务 CRUD ======
			secured.GET("/maintenance/tasks", h.ListMaintenanceTasks)
			secured.POST("/maintenance/tasks", h.CreateMaintenanceTask)
			secured.GET("/maintenance/tasks/:id", h.GetMaintenanceTask)
			secured.PUT("/maintenance/tasks/:id", h.UpdateMaintenanceTask)
			secured.DELETE("/maintenance/tasks/:id", h.DeleteMaintenanceTask)
			secured.PATCH("/maintenance/tasks/:id/status", h.UpdateMaintenanceStatus)

			// ====== 周期维护规则 ======
			secured.GET("/maintenance/rules", h.ListMaintenanceRules)
			secured.POST("/maintenance/rules", h.CreateMaintenanceRule)
			secured.GET("/maintenance/rules/:id", h.GetMaintenanceRule)
			secured.PATCH("/maintenance/rules/:id/status", h.ToggleMaintenanceRule)

			// ====== 冲突检测 & 改约建议 ======
			secured.POST("/maintenance/check-conflicts", h.CheckMaintenanceConflicts)
			secured.POST("/maintenance/reschedule-suggestions", h.GetRescheduleSuggestions)

			// ====== 冲突解决：改约 / 退订 ======
			secured.POST("/maintenance/resolve-conflict", h.ResolveSingleConflict)
			secured.POST("/maintenance/batch-resolve", h.BatchResolveConflicts)

			// ====== 维护日历 & 空档 ======
			secured.GET("/maintenance/calendar", h.MaintenanceCalendar)

			// ====== 统计分析 ======
			secured.GET("/maintenance/stats", h.MaintenanceStats)

			// ====== 冲突记录审计 ======
			secured.GET("/maintenance/conflicts", h.ListMaintenanceConflicts)
		}
	}

	log.Printf("venue-booking-admin listening on :%s", cfg.Port)
	if err := r.Run("0.0.0.0:" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
