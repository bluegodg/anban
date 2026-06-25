package outbox

import (
	"context"
	"log"
	"time"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

const (
	// DefaultTTL：一条主动播报排队多久没补播就过期（避免老旧问候深夜补播）。
	DefaultTTL = 2 * time.Hour
	// DefaultMaxPerDevice：单设备待播上限，超出丢最旧（避免一次补播一长串）。
	DefaultMaxPerDevice = 20
)

// repository 抽象持久化，便于测试替身。
type repository interface {
	Enqueue(ctx context.Context, item *Item) error
	CountPending(ctx context.Context, deviceID string) (int64, error)
	ListPending(ctx context.Context, deviceID string, limit int) ([]Item, error)
	PendingDeviceIDs(ctx context.Context) ([]string, error)
	Save(ctx context.Context, item *Item) error
	MarkDelivered(ctx context.Context, item *Item, at time.Time) error
	ExpirePending(ctx context.Context, deviceID string, now time.Time) (int, error)
}

// Service 负责入队与补播。补播一律走真实 client（非装饰器），否则会自环。
type Service struct {
	inner        xiaozhiclient.Client
	store        repository
	now          func() time.Time
	ttl          time.Duration
	maxPerDevice int
}

func NewService(inner xiaozhiclient.Client, st repository) *Service {
	return &Service{
		inner:        inner,
		store:        st,
		now:          func() time.Time { return time.Now().UTC() },
		ttl:          DefaultTTL,
		maxPerDevice: DefaultMaxPerDevice,
	}
}

// SetClock 仅供测试。
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// SetMaxPerDevice 仅供测试调小上限。
func (s *Service) SetMaxPerDevice(n int) {
	if n > 0 {
		s.maxPerDevice = n
	}
}

// Enqueue 把一条主动播报放入待播队列（装饰器在冷/热未知时统一调它）。
func (s *Service) Enqueue(ctx context.Context, deviceID, text string, opts xiaozhiclient.InjectOptions) error {
	now := s.now().UTC()
	item := &Item{
		DeviceID:   deviceID,
		Text:       text,
		SkipLLM:    opts.SkipLLM,
		AutoListen: opts.AutoListen,
		Status:     StatusPending,
		CreatedAt:  now,
		ExpiresAt:  now.Add(s.ttl),
	}
	if err := s.store.Enqueue(ctx, item); err != nil {
		return err
	}
	s.trimToCap(ctx, deviceID, now)
	return nil
}

// trimToCap 超出上限时把最旧的待播标记过期。
func (s *Service) trimToCap(ctx context.Context, deviceID string, now time.Time) {
	n, err := s.store.CountPending(ctx, deviceID)
	if err != nil || int(n) <= s.maxPerDevice {
		return
	}
	excess := int(n) - s.maxPerDevice
	items, err := s.store.ListPending(ctx, deviceID, excess)
	if err != nil {
		return
	}
	for i := range items {
		items[i].Status = StatusExpired
		items[i].LastError = "超出待播上限，丢弃最旧"
		_ = s.store.Save(ctx, &items[i])
	}
}

// Flush 把某设备所有未过期待播按序补播；只在调用方确认设备"热"时调用。
// 返回成功投递条数。
func (s *Service) Flush(ctx context.Context, deviceID string) (int, error) {
	now := s.now().UTC()
	if _, err := s.store.ExpirePending(ctx, deviceID, now); err != nil {
		return 0, err
	}
	items, err := s.store.ListPending(ctx, deviceID, s.maxPerDevice)
	if err != nil {
		return 0, err
	}
	delivered := 0
	for i := range items {
		it := items[i]
		err := s.inner.InjectSpeak(ctx, deviceID, it.Text, xiaozhiclient.InjectOptions{
			SkipLLM:    it.SkipLLM,
			AutoListen: it.AutoListen,
		})
		if err != nil {
			it.Attempts++
			it.LastError = err.Error()
			_ = s.store.Save(ctx, &it)
			log.Printf("outbox 补播失败 device=%s item=%d: %v（保留待播）", deviceID, it.ID, err)
			continue
		}
		if err := s.store.MarkDelivered(ctx, &it, s.now().UTC()); err != nil {
			log.Printf("outbox 标记已投递失败 device=%s item=%d: %v", deviceID, it.ID, err)
			continue
		}
		delivered++
	}
	if delivered > 0 {
		log.Printf("outbox 补播成功 device=%s 投递=%d 条", deviceID, delivered)
	}
	return delivered, nil
}

// PendingDeviceIDs 暴露给补播轮询：哪些设备还有待播。
func (s *Service) PendingDeviceIDs(ctx context.Context) ([]string, error) {
	return s.store.PendingDeviceIDs(ctx)
}
