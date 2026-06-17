package main

import (
	"context"
	"fmt"
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
	"github.com/bluegodg/anban/server/internal/llm"
	"github.com/bluegodg/anban/server/internal/memory"
	"github.com/bluegodg/anban/server/internal/mind"
	"github.com/bluegodg/anban/server/internal/mind/engine"
	"github.com/bluegodg/anban/server/internal/mind/executors"
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
	// 留言是子女主动发的点对点消息，不走"主动语音10分钟配额"（配额只给问候/提醒/视觉等自主播报防聒噪），发了就直接播。
	messageService := message.NewService(messageStore, xc)
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

	memoryStore := memory.NewStore(st.DB)
	if err := memoryStore.AutoMigrate(); err != nil {
		log.Fatalf("memory 表迁移失败: %v", err)
	}
	var factExtractor llm.FactExtractor
	if cfg.LLM.Enabled() {
		factExtractor = llm.NewArkClient(llm.ArkConfig{
			BaseURL: cfg.LLM.BaseURL,
			APIKey:  cfg.LLM.APIKey,
			Model:   cfg.LLM.Model,
		})
	} else {
		log.Printf("memory distill disabled: ANBAN_LLM_BASE_URL/API_KEY/MODEL 未完整配置，保持只画像注入")
	}
	memoryService := memory.NewService(memoryStore, xc, factExtractor, profileService, memory.Options{})
	if factExtractor != nil && cfg.MemoryDistillCron != "" {
		if _, err := sch.RegisterCron(cfg.MemoryDistillCron, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			deviceIDs, err := profileStore.ListDeviceIDs(ctx)
			if err != nil {
				log.Printf("memory distill 获取设备列表失败: %v", err)
				return
			}
			for _, deviceID := range deviceIDs {
				result, err := memoryService.DistillDevice(ctx, deviceID)
				if err != nil {
					log.Printf("memory distill 失败 device=%s: %v", deviceID, err)
					continue
				}
				if result.AddedFacts > 0 {
					log.Printf("memory distill 完成 device=%s 新增事实=%d 总事实=%d", result.DeviceID, result.AddedFacts, result.TotalFacts)
				}
			}
		}); err != nil {
			log.Fatalf("memory distill 调度失败: %v", err)
		}
	}

	mindStore := mind.NewStore(st.DB)
	if err := mindStore.AutoMigrate(); err != nil {
		log.Fatalf("mind 表迁移失败: %v", err)
	}
	mindEngine := engine.New(mindStore)
	_ = mindEngine

	visionService := vision.NewService(xc, greetingService)
	startVisionPresencePoller(sch, cfg.VisionPresenceInterval, profileStore, visionService)
	visionHandler := vision.NewHandler(visionService)

	mindDispatcher := executors.NewDispatcher(map[string]executors.SpeakExecutor{
		"message": executors.SpeakFunc(func(ctx context.Context, action mind.Action) (executors.Result, error) {
			id, _ := action.Args["messageId"].(float64)
			if id <= 0 {
				return executors.Result{ActionID: action.ID, Status: mind.ActionFailed, ErrorMessage: "messageId missing"}, nil
			}
			msg, err := messageService.PlayQueued(ctx, uint(id))
			if err != nil {
				return executors.Result{ActionID: action.ID, Status: mind.ActionFailed, ErrorMessage: err.Error()}, err
			}
			return executors.Result{ActionID: action.ID, Status: mind.ActionExecuted, ExecutorRef: fmt.Sprintf("message:%d", msg.ID)}, nil
		}),
		"greeting": executors.SpeakFunc(func(ctx context.Context, action mind.Action) (executors.Result, error) {
			greeting, err := greetingService.SpeakText(ctx, action.DeviceID, action.Text)
			if err != nil {
				return executors.Result{ActionID: action.ID, Status: mind.ActionFailed, ErrorMessage: err.Error()}, err
			}
			return executors.Result{ActionID: action.ID, Status: mind.ActionExecuted, ExecutorRef: fmt.Sprintf("greeting:%d", greeting.ID)}, nil
		}),
	})
	_ = mindDispatcher

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

func startVisionPresencePoller(sch *scheduler.Scheduler, interval time.Duration, profileStore *profile.Store, visionService *vision.Service) {
	if interval <= 0 {
		log.Printf("vision presence poller disabled: ANBAN_VISION_PRESENCE_INTERVAL=%s", interval)
		return
	}

	var scheduleNext func()
	scheduleNext = func() {
		if _, err := sch.ScheduleAt(time.Now().Add(interval), func() {
			runVisionPresencePoll(profileStore, visionService)
			scheduleNext()
		}); err != nil {
			log.Printf("vision presence poller 调度失败: %v", err)
		}
	}
	scheduleNext()
	log.Printf("vision presence poller enabled: interval=%s", interval)
}

func runVisionPresencePoll(profileStore *profile.Store, visionService *vision.Service) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deviceIDs, err := profileStore.ListDeviceIDs(ctx)
	if err != nil {
		log.Printf("vision presence 获取设备列表失败: %v", err)
		return
	}
	for _, deviceID := range deviceIDs {
		result, err := visionService.PollPresence(ctx, deviceID)
		if err != nil {
			log.Printf("vision presence 检测失败 device=%s: %v", deviceID, err)
			continue
		}
		if result.Skipped {
			continue
		}
		if result.Check.Observation.TriggeredGreeting {
			log.Printf("vision presence 触发问候 device=%s", result.DeviceID)
		}
	}
}
