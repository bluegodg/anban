package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bluegodg/anban/server/internal/childapi"
	"github.com/bluegodg/anban/server/internal/config"
	"github.com/bluegodg/anban/server/internal/domains/account"
	"github.com/bluegodg/anban/server/internal/domains/devicebinding"
	"github.com/bluegodg/anban/server/internal/domains/greeting"
	"github.com/bluegodg/anban/server/internal/domains/message"
	"github.com/bluegodg/anban/server/internal/domains/profile"
	"github.com/bluegodg/anban/server/internal/domains/reminder"
	"github.com/bluegodg/anban/server/internal/domains/status"
	"github.com/bluegodg/anban/server/internal/domains/timeline"
	"github.com/bluegodg/anban/server/internal/domains/vision"
	"github.com/bluegodg/anban/server/internal/llm"
	"github.com/bluegodg/anban/server/internal/memory"
	"github.com/bluegodg/anban/server/internal/mind"
	"github.com/bluegodg/anban/server/internal/mind/engine"
	"github.com/bluegodg/anban/server/internal/mind/executors"
	"github.com/bluegodg/anban/server/internal/mind/promptctx"
	"github.com/bluegodg/anban/server/internal/openmemory"
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

	accountStore := account.NewStore(st.DB)
	if err := accountStore.AutoMigrate(); err != nil {
		log.Fatalf("account 表迁移失败: %v", err)
	}
	accountService := account.NewService(accountStore, account.Options{
		DevVerificationCode: cfg.DevVerificationCode,
	})

	deviceBindingStore := devicebinding.NewStore(st.DB)
	if err := deviceBindingStore.AutoMigrate(); err != nil {
		log.Fatalf("devicebinding 表迁移失败: %v", err)
	}
	deviceBindingService := devicebinding.NewService(deviceBindingStore, devicebinding.Options{})
	if _, err := deviceBindingService.EnsureDevice(context.Background(), devicebinding.DeviceSeed{
		DeviceID:         cfg.DemoDeviceID,
		BindingCode:      cfg.DemoBindingCode,
		DisplayName:      cfg.DemoDeviceDisplayName,
		ElderDisplayName: cfg.DemoElderDisplayName,
	}); err != nil {
		log.Fatalf("demo 设备初始化失败: %v", err)
	}

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
	var arkClient *llm.ArkClient
	if cfg.LLM.Enabled() {
		arkClient = llm.NewArkClient(llm.ArkConfig{
			BaseURL: cfg.LLM.BaseURL,
			APIKey:  cfg.LLM.APIKey,
			Model:   cfg.LLM.Model,
		})
	} else {
		log.Printf("memory distill and AI portrait disabled: ANBAN_LLM_BASE_URL/API_KEY/MODEL 未完整配置，保留管理员手动记忆和画像")
	}
	var portraitGenerator profile.PortraitGenerator
	if arkClient != nil {
		portraitGenerator = profilePortraitGenerator{client: arkClient}
	}
	profileService := profile.NewService(profileStore, portraitGenerator)
	profileHandler := profile.NewHandler(profileService)
	timelineService := timeline.NewService(messageService, xc, profileService)
	timelineHandler := timeline.NewHandler(timelineService)

	memoryStore := memory.NewStore(st.DB)
	if err := memoryStore.AutoMigrate(); err != nil {
		log.Fatalf("memory 表迁移失败: %v", err)
	}
	var factExtractor llm.FactExtractor
	if arkClient != nil {
		factExtractor = arkClient
	}
	memoryService := memory.NewService(memoryStore, xc, factExtractor, profileService, memory.Options{})
	memoryHandler := memory.NewHandler(memoryService)
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
	mindEngine.UseCompanionContextReader(profileCompanionContextReader{profiles: profileService})
	configureMindEngine(mindEngine, cfg)
	messageService.UseMindSink(messageMindSink{engine: mindEngine})
	reminderService.UseMindSink(reminderMindSink{engine: mindEngine})

	visionStore := vision.NewStore(st.DB)
	if err := visionStore.AutoMigrate(); err != nil {
		log.Fatalf("vision 表迁移失败: %v", err)
	}
	visionService := vision.NewService(xc, greetingService)
	visionService.UseStore(visionStore)
	visionService.UseMediaRoot(cfg.VisionMediaRoot)
	visionService.UseCaptureTimeout(cfg.VisionCaptureTimeout)
	visionService.UseRetentionDays(cfg.VisionRetentionDays)
	visionService.UseMaxCapturesPerDevice(cfg.VisionMaxCapturesPerDevice)
	if cfg.XiaozhiVisionURL != "" {
		visionService.UseVisionForwarder(xiaozhiclient.NewVisionForwarder(cfg.XiaozhiVisionURL))
	} else {
		log.Printf("vision device proxy disabled: ANBAN_XIAOZHI_VISION_URL 未配置")
	}
	visionService.UseMindSink(visionMindSink{engine: mindEngine})
	startVisionPresencePoller(sch, cfg.VisionPresenceInterval, profileStore, visionService)
	startVisionCaptureMaintenance(sch, time.Minute, visionService)
	visionHandler := vision.NewHandler(visionService)

	mindDispatcher := executors.NewDispatcher(map[string]executors.SpeakExecutor{
		"greeting": newMindGreetingSpeakExecutor(greetingService),
	})
	mindEngine.UseExecutor(mindActionExecutor{dispatcher: mindDispatcher})
	startMindLoops(sch, profileStore, mindEngine, profileService, cfg.MindLoopInterval)
	startMindHistoryPoller(sch, cfg.MindHistoryInterval, profileStore, xc, mindEngine)
	if portraitGenerator != nil {
		startAIPortraitRefresh(sch, profileStore, profileService, mindEngine, profileService, 5*time.Second)
	}

	r := childapi.NewRouter(childapi.Deps{
		AccessCode:           cfg.AccessCode,
		AllowedOrigins:       cfg.AllowedOrigins,
		AccountService:       accountService,
		DeviceBindingService: deviceBindingService,
		MessageRoutes:        messageHandler,
		GreetingRoutes:       greetingHandler,
		ReminderRoutes:       reminderHandler,
		StatusRoutes:         statusHandler,
		ProfileRoutes:        profileHandler,
		MemoryRoutes:         memoryHandler,
		VisionRoutes:         visionHandler,
		TimelineRoutes:       timelineHandler,
	})
	if cfg.MemoryProviderToken != "" {
		openmemory.NewHandler(cfg.MemoryProviderToken, cfg.DemoDeviceID, profileService).RegisterRoutes(r.Group("/api/openmem/v1"))
		log.Printf("open memory provider enabled: /api/openmem/v1")
	} else {
		log.Printf("open memory provider disabled: ANBAN_MEMORY_PROVIDER_TOKEN 未配置")
	}
	visionHandler.RegisterDeviceRoutes(r.Group("/api"), cfg.DeviceVisionToken)

	log.Printf("anban 启动，监听 %s（manager=%s）", cfg.ListenAddr, cfg.ManagerBaseURL)
	if err := r.Run(cfg.ListenAddr); err != nil {
		log.Fatalf("HTTP 服务退出: %v", err)
	}
}

type mindEngineConfigTarget interface {
	UseLocation(*time.Location)
	UseProactiveCooldown(time.Duration)
	UseProactiveDaytimeOnly(bool)
}

func configureMindEngine(target mindEngineConfigTarget, cfg config.Config) {
	target.UseLocation(cfg.TimezoneLocation)
	target.UseProactiveCooldown(cfg.MindProactiveCooldown)
	target.UseProactiveDaytimeOnly(cfg.MindProactiveDaytimeOnly)
}

type mindGreetingSpeaker interface {
	SpeakText(ctx context.Context, deviceID, text string) (greeting.Greeting, error)
}

func newMindGreetingSpeakExecutor(speaker mindGreetingSpeaker) executors.SpeakExecutor {
	return executors.SpeakFunc(func(ctx context.Context, action mind.Action) (executors.Result, error) {
		spokenGreeting, err := speaker.SpeakText(ctx, action.DeviceID, action.Text)
		result := greetingSpeakResult(action, spokenGreeting, err)
		if err != nil && isMindProactiveAction(action) {
			if result.Status == mind.ActionFailed {
				result.Status = mind.ActionDeferred
			}
			if result.ErrorMessage == "" {
				result.ErrorMessage = err.Error()
			}
			return result, nil
		}
		if err != nil {
			return result, err
		}
		if result.Status == mind.ActionFailed && isMindProactiveAction(action) {
			result.Status = mind.ActionDeferred
		}
		return result, nil
	})
}

func greetingSpeakResult(action mind.Action, spokenGreeting greeting.Greeting, err error) executors.Result {
	result := executors.Result{ActionID: action.ID}
	if spokenGreeting.ID > 0 {
		result.ExecutorRef = fmt.Sprintf("greeting:%d", spokenGreeting.ID)
	}
	if err != nil {
		result.Status = mind.ActionFailed
		result.ErrorMessage = err.Error()
		return result
	}
	switch spokenGreeting.Status {
	case greeting.StatusPlayed:
		result.Status = mind.ActionExecuted
	case greeting.StatusPending:
		result.Status = mind.ActionDeferred
		result.ErrorMessage = spokenGreeting.ErrorMessage
	default:
		result.Status = mind.ActionFailed
		result.ErrorMessage = spokenGreeting.ErrorMessage
	}
	return result
}

func isMindProactiveAction(action mind.Action) bool {
	value, _ := action.Args["mindProactive"].(bool)
	return value
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

type visionCaptureMaintainer interface {
	FinalizeTimedOutCaptures(ctx context.Context, now time.Time) (int, error)
	ExpireCaptures(ctx context.Context, now time.Time) (int, error)
	PruneExcessCaptures(ctx context.Context) (int, error)
}

func startVisionCaptureMaintenance(sch *scheduler.Scheduler, interval time.Duration, maintainer visionCaptureMaintainer) {
	if interval <= 0 {
		interval = time.Minute
	}
	var scheduleNext func()
	scheduleNext = func() {
		if _, err := sch.ScheduleAt(time.Now().Add(interval), func() {
			runVisionCaptureMaintenance(maintainer, time.Now().UTC())
			scheduleNext()
		}); err != nil {
			log.Printf("vision capture maintenance 调度失败: %v", err)
		}
	}
	scheduleNext()
	log.Printf("vision capture maintenance enabled: interval=%s", interval)
}

func runVisionCaptureMaintenance(maintainer visionCaptureMaintainer, now time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if count, err := maintainer.FinalizeTimedOutCaptures(ctx, now); err != nil {
		log.Printf("vision capture 超时终结失败: %v", err)
	} else if count > 0 {
		log.Printf("vision capture 超时终结 %d 条", count)
	}
	if count, err := maintainer.ExpireCaptures(ctx, now); err != nil {
		log.Printf("vision capture 过期清理失败: %v", err)
	} else if count > 0 {
		log.Printf("vision capture 过期清理 %d 条", count)
	}
	if count, err := maintainer.PruneExcessCaptures(ctx); err != nil {
		log.Printf("vision capture 数量清理失败: %v", err)
	} else if count > 0 {
		log.Printf("vision capture 数量清理 %d 条", count)
	}
}

type mindContextSyncer interface {
	SyncMindContext(ctx context.Context, deviceID string, mindContext string) error
}

type profileCompanionContextReader struct {
	profiles *profile.Service
}

func (r profileCompanionContextReader) CompanionContext(ctx context.Context, deviceID string) (promptctx.CompanionContext, error) {
	if r.profiles == nil {
		return promptctx.CompanionContext{}, nil
	}
	current, err := r.profiles.Get(ctx, deviceID)
	if errors.Is(err, profile.ErrNotFound) {
		return promptctx.CompanionContext{}, nil
	}
	if err != nil {
		return promptctx.CompanionContext{}, err
	}

	displayName := strings.TrimSpace(current.Fields.Nickname)
	if displayName == "" {
		displayName = strings.TrimSpace(current.Fields.Name)
	}
	return promptctx.CompanionContext{
		DisplayName:      displayName,
		ProfileSummaries: profileSummaries(current.Fields),
		MemoryFacts:      current.MemoryFacts,
	}, nil
}

func profileSummaries(fields profile.Fields) []string {
	summaries := []string{}
	add := func(label, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			summaries = append(summaries, label+"："+value)
		}
	}
	add("AI认知画像", fields.AIPortrait)
	add("老人本名", fields.Name)
	add("常用称呼", fields.Nickname)
	add("子女", strings.Join(fields.Children, "、"))
	add("孙辈", strings.Join(fields.Grandchildren, "、"))
	add("喜好", strings.Join(fields.Hobbies, "、"))
	add("作息", fields.Schedule)
	add("健康背景", fields.Health)
	add("忌口和禁忌", strings.Join(fields.Taboos, "、"))
	return summaries
}

type portraitLLM interface {
	GeneratePortrait(ctx context.Context, req llm.PortraitRequest) (string, error)
}

type profilePortraitGenerator struct {
	client portraitLLM
}

func (g profilePortraitGenerator) GeneratePortrait(ctx context.Context, input profile.PortraitInput) (string, error) {
	fields := input.Fields
	fields.AIPortrait = ""
	fields.AIPortraitMode = ""
	return g.client.GeneratePortrait(ctx, llm.PortraitRequest{
		ProfileContext:   profile.BuildPrompt(fields),
		MemoryFacts:      input.MemoryFacts,
		PreviousPortrait: input.PreviousPortrait,
	})
}

type aiPortraitRefresher interface {
	RefreshAIPortrait(ctx context.Context, deviceID string) (profile.Profile, error)
}

func startAIPortraitRefresh(
	sch *scheduler.Scheduler,
	profileStore *profile.Store,
	refresher aiPortraitRefresher,
	mindEngine mind.Engine,
	mindContextSyncer mindContextSyncer,
	delay time.Duration,
) {
	if refresher == nil {
		return
	}
	if delay <= 0 {
		delay = 5 * time.Second
	}
	if _, err := sch.ScheduleAt(time.Now().Add(delay), func() {
		runAIPortraitRefreshThenMindSync(profileStore, refresher, mindEngine, mindContextSyncer)
	}); err != nil {
		log.Printf("AI portrait refresh 调度失败: %v", err)
	}
}

func runAIPortraitRefreshThenMindSync(
	profileStore *profile.Store,
	refresher aiPortraitRefresher,
	mindEngine mind.Engine,
	mindContextSyncer mindContextSyncer,
) {
	runAIPortraitRefresh(profileStore, refresher)
	runMindContextSync(profileStore, mindEngine, mindContextSyncer)
}

func runAIPortraitRefresh(profileStore *profile.Store, refresher aiPortraitRefresher) {
	if refresher == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	deviceIDs, err := profileStore.ListDeviceIDs(ctx)
	if err != nil {
		log.Printf("AI portrait refresh 获取设备列表失败: %v", err)
		return
	}
	for _, deviceID := range deviceIDs {
		if _, err := refresher.RefreshAIPortrait(ctx, deviceID); err != nil {
			log.Printf("AI portrait refresh 失败 device=%s: %v", deviceID, err)
		}
	}
}

func startMindLoops(
	sch *scheduler.Scheduler,
	profileStore *profile.Store,
	mindEngine mind.Engine,
	mindContextSyncer mindContextSyncer,
	interval time.Duration,
) {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	runMindContextSync(profileStore, mindEngine, mindContextSyncer)

	var scheduleNext func()
	scheduleNext = func() {
		if _, err := sch.ScheduleAt(time.Now().Add(interval), func() {
			runMindLoops(profileStore, mindEngine, mindContextSyncer)
			scheduleNext()
		}); err != nil {
			log.Printf("mind loops 调度失败: %v", err)
		}
	}
	scheduleNext()
	log.Printf("mind loops enabled: interval=%s", interval)
}

func runMindContextSync(profileStore *profile.Store, mindEngine mind.Engine, mindContextSyncer mindContextSyncer) {
	if mindContextSyncer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deviceIDs, err := profileStore.ListDeviceIDs(ctx)
	if err != nil {
		log.Printf("mind context 获取设备列表失败: %v", err)
		return
	}
	now := time.Now().UTC()
	for _, deviceID := range deviceIDs {
		syncMindContext(ctx, mindEngine, mindContextSyncer, deviceID, now)
	}
}

func runMindLoops(profileStore *profile.Store, mindEngine mind.Engine, mindContextSyncer mindContextSyncer) {
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

		syncMindContext(ctx, mindEngine, mindContextSyncer, deviceID, now)
	}
}

func syncMindContext(ctx context.Context, mindEngine mind.Engine, mindContextSyncer mindContextSyncer, deviceID string, now time.Time) {
	if mindContextSyncer == nil {
		return
	}
	mindContext, err := mindEngine.BuildMindContext(ctx, deviceID, now)
	if err != nil {
		log.Printf("mind context 生成失败 device=%s: %v", deviceID, err)
		return
	}
	mindContext = strings.TrimSpace(mindContext)
	if mindContext == "" {
		return
	}
	if err := mindContextSyncer.SyncMindContext(ctx, deviceID, mindContext); err != nil {
		log.Printf("mind context 同步失败 device=%s: %v", deviceID, err)
	}
}

const mindHistoryLimit = 50

func startMindHistoryPoller(
	sch *scheduler.Scheduler,
	interval time.Duration,
	profileStore *profile.Store,
	xc xiaozhiclient.Client,
	mindEngine mind.Engine,
) {
	if interval <= 0 {
		interval = time.Minute
	}

	var scheduleNext func()
	scheduleNext = func() {
		if _, err := sch.ScheduleAt(time.Now().Add(interval), func() {
			runMindHistoryPoll(profileStore, xc, mindEngine)
			scheduleNext()
		}); err != nil {
			log.Printf("mind history poller 调度失败: %v", err)
		}
	}
	scheduleNext()
	log.Printf("mind history poller enabled: interval=%s", interval)
}

func runMindHistoryPoll(profileStore *profile.Store, xc xiaozhiclient.Client, mindEngine mind.Engine) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deviceIDs, err := profileStore.ListDeviceIDs(ctx)
	if err != nil {
		log.Printf("mind history 获取设备列表失败: %v", err)
		return
	}
	for _, deviceID := range deviceIDs {
		history, err := xc.GetHistory(ctx, deviceID, mindHistoryLimit)
		if err != nil {
			log.Printf("mind history 读取失败 device=%s: %v", deviceID, err)
			continue
		}
		for _, message := range history {
			event, ok := historyMindEvent(deviceID, message)
			if !ok {
				continue
			}
			if _, err := mindEngine.Ingest(ctx, event); err != nil {
				log.Printf("mind history 进入心智失败 device=%s event=%s: %v", deviceID, event.ID, err)
			}
		}
	}
}

func historyMindEvent(deviceID string, message xiaozhiclient.HistoryMessage) (mind.Event, bool) {
	deviceID = strings.TrimSpace(deviceID)
	role := strings.ToLower(strings.TrimSpace(message.Role))
	text := strings.TrimSpace(message.Text)
	if deviceID == "" || text == "" || message.At.IsZero() {
		return mind.Event{}, false
	}

	var eventType mind.EventType
	var salience float64
	switch role {
	case "user":
		eventType = mind.EventElderSpoke
		salience = 0.55
	case "assistant":
		eventType = mind.EventAssistantSpoke
		salience = 0.45
	default:
		return mind.Event{}, false
	}

	at := message.At.UTC()
	return mind.Event{
		ID:       historyEventID(deviceID, role, at, text),
		DeviceID: deviceID,
		Type:     eventType,
		Source:   mind.SourceXiaozhi,
		At:       at,
		Summary:  conversationEventSummary(role, text),
		Payload: map[string]any{
			"role": role,
			"text": text,
		},
		Salience:   salience,
		Emotion:    "conversational",
		Confidence: 0.85,
	}, true
}

func historyEventID(deviceID, role string, at time.Time, text string) string {
	sum := sha256.Sum256([]byte(deviceID + "|" + role + "|" + at.UTC().Format(time.RFC3339Nano) + "|" + text))
	return fmt.Sprintf("evt-xiaozhi-%s-%s-%s", deviceID, role, hex.EncodeToString(sum[:8]))
}

func conversationEventSummary(role, text string) string {
	prefix := "老人说："
	if role == "assistant" {
		prefix = "安伴回应："
	}
	return prefix + truncateSummaryRunes(text, 80)
}

func truncateSummaryRunes(value string, limit int) string {
	runes := []rune(value)
	if limit <= 0 || len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
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
	if errors.Is(err, executors.ErrExecutorNotFound) {
		return engine.ExecutionResult{
			Status:       mind.ActionDeferred,
			ErrorMessage: err.Error(),
		}, nil
	}
	return engine.ExecutionResult{
		Status:       result.Status,
		ExecutorRef:  result.ExecutorRef,
		ErrorMessage: result.ErrorMessage,
	}, err
}
