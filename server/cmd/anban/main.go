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
	messageService.UseMindSink(messageMindSink{engine: mindEngine})
	reminderService.UseMindSink(reminderMindSink{engine: mindEngine})

	visionService := vision.NewService(xc, greetingService)
	visionService.UseMindSink(visionMindSink{engine: mindEngine})
	startVisionPresencePoller(sch, cfg.VisionPresenceInterval, profileStore, visionService)
	visionHandler := vision.NewHandler(visionService)

	mindDispatcher := executors.NewDispatcher(map[string]executors.SpeakExecutor{
		"message": executors.SpeakFunc(func(ctx context.Context, action mind.Action) (executors.Result, error) {
			id, ok := uintArg(action.Args, "messageId")
			if !ok {
				err := fmt.Errorf("messageId missing")
				return executors.Result{ActionID: action.ID, Status: mind.ActionFailed, ErrorMessage: err.Error()}, err
			}
			msg, err := messageService.PlayQueued(ctx, id)
			if err != nil {
				return executors.Result{ActionID: action.ID, Status: mind.ActionFailed, ErrorMessage: err.Error()}, err
			}
			if msg.Status != message.StatusPlayed {
				return executors.Result{ActionID: action.ID, Status: mind.ActionFailed, ErrorMessage: msg.ErrorMessage}, nil
			}
			return executors.Result{ActionID: action.ID, Status: mind.ActionExecuted, ExecutorRef: fmt.Sprintf("message:%d", msg.ID)}, nil
		}),
		"reminder": executors.SpeakFunc(func(ctx context.Context, action mind.Action) (executors.Result, error) {
			id, ok := uintArg(action.Args, "reminderId")
			if !ok {
				err := fmt.Errorf("reminderId missing")
				return executors.Result{ActionID: action.ID, Status: mind.ActionFailed, ErrorMessage: err.Error()}, err
			}
			rem, err := reminderService.PlayScheduled(ctx, id)
			if err != nil {
				return executors.Result{ActionID: action.ID, Status: mind.ActionFailed, ErrorMessage: err.Error()}, err
			}
			switch rem.Status {
			case reminder.StatusPlayed:
				return executors.Result{ActionID: action.ID, Status: mind.ActionExecuted, ExecutorRef: fmt.Sprintf("reminder:%d", rem.ID)}, nil
			case reminder.StatusScheduled:
				return executors.Result{ActionID: action.ID, Status: mind.ActionDeferred, ExecutorRef: fmt.Sprintf("reminder:%d", rem.ID), ErrorMessage: rem.ErrorMessage}, nil
			default:
				return executors.Result{ActionID: action.ID, Status: mind.ActionFailed, ExecutorRef: fmt.Sprintf("reminder:%d", rem.ID), ErrorMessage: rem.ErrorMessage}, nil
			}
		}),
		"greeting": executors.SpeakFunc(func(ctx context.Context, action mind.Action) (executors.Result, error) {
			spokenGreeting, err := greetingService.SpeakText(ctx, action.DeviceID, action.Text)
			if err != nil {
				return executors.Result{ActionID: action.ID, Status: mind.ActionFailed, ErrorMessage: err.Error()}, err
			}
			switch spokenGreeting.Status {
			case greeting.StatusPlayed:
				return executors.Result{ActionID: action.ID, Status: mind.ActionExecuted, ExecutorRef: fmt.Sprintf("greeting:%d", spokenGreeting.ID)}, nil
			case greeting.StatusPending:
				return executors.Result{ActionID: action.ID, Status: mind.ActionDeferred, ExecutorRef: fmt.Sprintf("greeting:%d", spokenGreeting.ID), ErrorMessage: spokenGreeting.ErrorMessage}, nil
			default:
				return executors.Result{ActionID: action.ID, Status: mind.ActionFailed, ExecutorRef: fmt.Sprintf("greeting:%d", spokenGreeting.ID), ErrorMessage: spokenGreeting.ErrorMessage}, nil
			}
		}),
	})
	mindEngine.UseExecutor(mindActionExecutor{dispatcher: mindDispatcher})
	startMindLoops(sch, profileStore, mindEngine, 15*time.Minute)

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

func startMindLoops(sch *scheduler.Scheduler, profileStore *profile.Store, mindEngine mind.Engine, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Minute
	}

	var scheduleNext func()
	scheduleNext = func() {
		if _, err := sch.ScheduleAt(time.Now().Add(interval), func() {
			runMindLoops(profileStore, mindEngine)
			scheduleNext()
		}); err != nil {
			log.Printf("mind loops 调度失败: %v", err)
		}
	}
	scheduleNext()
	log.Printf("mind loops enabled: interval=%s", interval)
}

func runMindLoops(profileStore *profile.Store, mindEngine mind.Engine) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deviceIDs, err := profileStore.ListDeviceIDs(ctx)
	if err != nil {
		log.Printf("mind loops 获取设备列表失败: %v", err)
		return
	}
	now := time.Now().UTC()
	for _, deviceID := range deviceIDs {
		if _, err := mindEngine.TickIdle(ctx, deviceID, now); err != nil {
			log.Printf("mind idle tick 失败 device=%s: %v", deviceID, err)
		}

		window := mind.TimeWindow{From: now.Add(-30 * time.Minute), To: now}
		if err := mindEngine.Reflect(ctx, deviceID, window); err != nil {
			log.Printf("mind reflection 失败 device=%s: %v", deviceID, err)
		}

		if err := mindEngine.UpdateLife(ctx, deviceID, now); err != nil {
			log.Printf("mind life update 失败 device=%s: %v", deviceID, err)
		}
	}
}

type messageMindSink struct {
	engine mind.Engine
}

func (s messageMindSink) IngestMindEvent(ctx context.Context, event message.MindEvent) error {
	_, err := s.engine.Ingest(ctx, mind.Event{
		DeviceID:   event.DeviceID,
		Type:       mind.EventType(event.Type),
		Source:     mind.SourceDomain,
		At:         time.Now().UTC(),
		Summary:    event.Summary,
		Payload:    event.Payload,
		Salience:   0.75,
		Emotion:    "warm",
		Confidence: 0.9,
	})
	return err
}

type reminderMindSink struct {
	engine mind.Engine
}

func (s reminderMindSink) IngestMindEvent(ctx context.Context, event reminder.MindEvent) error {
	_, err := s.engine.Ingest(ctx, mind.Event{
		DeviceID:   event.DeviceID,
		Type:       mind.EventType(event.Type),
		Source:     mind.SourceDomain,
		At:         time.Now().UTC(),
		Summary:    event.Summary,
		Payload:    event.Payload,
		Salience:   0.85,
		Emotion:    "caring",
		Confidence: 0.95,
	})
	return err
}

type visionMindSink struct {
	engine mind.Engine
}

func (s visionMindSink) IngestMindEvent(ctx context.Context, event vision.MindEvent) error {
	_, err := s.engine.Ingest(ctx, mind.Event{
		DeviceID:   event.DeviceID,
		Type:       mind.EventType(event.Type),
		Source:     mind.SourceVision,
		At:         time.Now().UTC(),
		Summary:    event.Summary,
		Payload:    event.Payload,
		Salience:   0.55,
		Emotion:    "warm",
		Confidence: 0.8,
	})
	return err
}

type mindActionExecutor struct {
	dispatcher *executors.Dispatcher
}

func (e mindActionExecutor) Execute(ctx context.Context, action mind.Action) (engine.ExecutionResult, error) {
	result, err := e.dispatcher.Execute(ctx, action)
	return engine.ExecutionResult{
		Status:       result.Status,
		ExecutorRef:  result.ExecutorRef,
		ErrorMessage: result.ErrorMessage,
	}, err
}

func uintArg(args map[string]any, key string) (uint, bool) {
	value, ok := args[key]
	if !ok {
		return 0, false
	}

	switch v := value.(type) {
	case uint:
		return positiveUint(v)
	case uint64:
		if v == 0 {
			return 0, false
		}
		return uint(v), true
	case uint32:
		return positiveUint(uint(v))
	case int:
		return positiveInt(v)
	case int64:
		return positiveInt64(v)
	case int32:
		return positiveInt(int(v))
	case float64:
		if v <= 0 {
			return 0, false
		}
		return uint(v), true
	case float32:
		if v <= 0 {
			return 0, false
		}
		return uint(v), true
	default:
		return 0, false
	}
}

func positiveUint(value uint) (uint, bool) {
	if value == 0 {
		return 0, false
	}
	return value, true
}

func positiveInt(value int) (uint, bool) {
	if value <= 0 {
		return 0, false
	}
	return uint(value), true
}

func positiveInt64(value int64) (uint, bool) {
	if value <= 0 {
		return 0, false
	}
	return uint(value), true
}
