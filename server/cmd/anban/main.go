package main

import (
	"context"
	"log"
	"time"

	"github.com/bluegodg/anban/server/internal/childapi"
	"github.com/bluegodg/anban/server/internal/config"
	"github.com/bluegodg/anban/server/internal/domains/greeting"
	"github.com/bluegodg/anban/server/internal/domains/message"
	"github.com/bluegodg/anban/server/internal/domains/profile"
	"github.com/bluegodg/anban/server/internal/domains/reminder"
	"github.com/bluegodg/anban/server/internal/domains/status"
	"github.com/bluegodg/anban/server/internal/domains/vision"
	"github.com/bluegodg/anban/server/internal/proactive"
	"github.com/bluegodg/anban/server/internal/scheduler"
	"github.com/bluegodg/anban/server/internal/store"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	st, err := store.Open(cfg.DBDSN)
	if err != nil {
		log.Fatalf("数据库打开失败: %v", err)
	}

	xc := xiaozhiclient.NewHTTPClient(cfg.ManagerBaseURL, cfg.ManagerAPIToken)

	sch := scheduler.New()
	sch.Start()
	defer sch.Stop()

	voiceGate := proactive.NewVoiceGate(10 * time.Minute)

	messageStore := message.NewStore(st.DB)
	if err := messageStore.AutoMigrate(); err != nil {
		log.Fatalf("message 表迁移失败: %v", err)
	}
	messageService := message.NewService(messageStore, xc, sch)
	messageService.UseProactiveVoiceGate(voiceGate)
	messageHandler := message.NewHandler(messageService)

	greetingStore := greeting.NewStore(st.DB)
	if err := greetingStore.AutoMigrate(); err != nil {
		log.Fatalf("greeting 表迁移失败: %v", err)
	}
	greetingService := greeting.NewService(greetingStore, xc, sch)
	greetingService.UseProactiveVoiceGate(voiceGate)
	if restored, err := greetingService.RestoreSchedules(context.Background()); err != nil {
		log.Fatalf("greeting 恢复调度失败: %v", err)
	} else if restored > 0 {
		log.Printf("greeting 恢复调度 %d 个时段", restored)
	}
	greetingHandler := greeting.NewHandler(greetingService)

	reminderStore := reminder.NewStore(st.DB)
	if err := reminderStore.AutoMigrate(); err != nil {
		log.Fatalf("reminder 表迁移失败: %v", err)
	}
	reminderService := reminder.NewService(reminderStore, xc, sch)
	reminderService.UseProactiveVoiceGate(voiceGate)
	if restored, err := reminderService.RestoreScheduled(context.Background()); err != nil {
		log.Fatalf("reminder 恢复调度失败: %v", err)
	} else if restored > 0 {
		log.Printf("reminder 恢复调度 %d 条", restored)
	}
	reminderHandler := reminder.NewHandler(reminderService)

	statusStore := status.NewStore(st.DB)
	if err := statusStore.AutoMigrate(); err != nil {
		log.Fatalf("status 表迁移失败: %v", err)
	}
	statusService := status.NewService(xc, messageService)
	statusService.UseStore(statusStore)
	statusHandler := status.NewHandler(statusService)

	profileStore := profile.NewStore(st.DB)
	if err := profileStore.AutoMigrate(); err != nil {
		log.Fatalf("profile 表迁移失败: %v", err)
	}
	profileService := profile.NewService(profileStore, xc)
	profileHandler := profile.NewHandler(profileService)

	visionService := vision.NewService(xc, greetingService)
	visionHandler := vision.NewHandler(visionService)

	r := childapi.NewRouter(childapi.Deps{
		AccessCode:     cfg.AccessCode,
		AllowedOrigins: cfg.AllowedOrigins,
		MessageRoutes:  messageHandler,
		GreetingRoutes: greetingHandler,
		ReminderRoutes: reminderHandler,
		StatusRoutes:   statusHandler,
		ProfileRoutes:  profileHandler,
		VisionRoutes:   visionHandler,
	})

	log.Printf("anban 启动，监听 %s（manager=%s）", cfg.ListenAddr, cfg.ManagerBaseURL)
	if err := r.Run(cfg.ListenAddr); err != nil {
		log.Fatalf("HTTP 服务退出: %v", err)
	}
}
