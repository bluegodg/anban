package main

import (
	"log"

	"github.com/bluegodg/anban/server/internal/childapi"
	"github.com/bluegodg/anban/server/internal/config"
	"github.com/bluegodg/anban/server/internal/domains/greeting"
	"github.com/bluegodg/anban/server/internal/domains/message"
	"github.com/bluegodg/anban/server/internal/domains/reminder"
	"github.com/bluegodg/anban/server/internal/domains/status"
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

	messageStore := message.NewStore(st.DB)
	if err := messageStore.AutoMigrate(); err != nil {
		log.Fatalf("message 表迁移失败: %v", err)
	}
	messageService := message.NewService(messageStore, xc)
	messageHandler := message.NewHandler(messageService)

	greetingStore := greeting.NewStore(st.DB)
	if err := greetingStore.AutoMigrate(); err != nil {
		log.Fatalf("greeting 表迁移失败: %v", err)
	}
	greetingService := greeting.NewService(greetingStore, xc)
	greetingHandler := greeting.NewHandler(greetingService)

	sch := scheduler.New()
	sch.Start()
	defer sch.Stop()

	reminderStore := reminder.NewStore(st.DB)
	if err := reminderStore.AutoMigrate(); err != nil {
		log.Fatalf("reminder 表迁移失败: %v", err)
	}
	reminderService := reminder.NewService(reminderStore, xc, sch)
	reminderHandler := reminder.NewHandler(reminderService)

	statusService := status.NewService(xc, messageService)
	statusHandler := status.NewHandler(statusService)

	r := childapi.NewRouter(childapi.Deps{
		AccessCode:     cfg.AccessCode,
		MessageRoutes:  messageHandler,
		GreetingRoutes: greetingHandler,
		ReminderRoutes: reminderHandler,
		StatusRoutes:   statusHandler,
	})

	log.Printf("anban 启动，监听 %s（manager=%s）", cfg.ListenAddr, cfg.ManagerBaseURL)
	if err := r.Run(cfg.ListenAddr); err != nil {
		log.Fatalf("HTTP 服务退出: %v", err)
	}
}
