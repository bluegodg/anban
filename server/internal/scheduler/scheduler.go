// Package scheduler 提供定时能力：cron 周期任务 + 一次性定时任务。
// 纪律：只提供机制；任务内容由各域以闭包传入。
package scheduler

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type JobID string

type Scheduler struct {
	cron     *cron.Cron
	mu       sync.Mutex
	oneShots map[JobID]*time.Timer
	seq      int64
}

// scheduleLocation 让所有 cron 定时统一按东八区（UTC+8）解释，避免容器默认 UTC
// 导致"早 8 点问候"在北京时间下午才触发。用 FixedZone 而非 LoadLocation：
// 中国无夏令时，且不依赖容器内 tzdata（alpine 等精简镜像也安全）。
var scheduleLocation = time.FixedZone("CST", 8*60*60)

func New() *Scheduler {
	return &Scheduler{
		cron:     cron.New(cron.WithLocation(scheduleLocation)),
		oneShots: make(map[JobID]*time.Timer),
	}
}

// Location 返回 cron 调度使用的时区（供装配/测试核对）。
func (s *Scheduler) Location() *time.Location {
	return s.cron.Location()
}

func (s *Scheduler) Start() { s.cron.Start() }
func (s *Scheduler) Stop() {
	s.cron.Stop()
	s.mu.Lock()
	for _, t := range s.oneShots {
		t.Stop()
	}
	s.oneShots = make(map[JobID]*time.Timer)
	s.mu.Unlock()
}

// RegisterCron 注册一个 cron 表达式周期任务（如每天 8 点："0 8 * * *"）。
func (s *Scheduler) RegisterCron(spec string, fn func()) (JobID, error) {
	eid, err := s.cron.AddFunc(spec, fn)
	if err != nil {
		return "", err
	}
	return JobID("cron-" + itoa(int64(eid))), nil
}

// ScheduleAt 在指定时刻触发一次 fn（用于一次性提醒）。
func (s *Scheduler) ScheduleAt(t time.Time, fn func()) (JobID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	id := JobID("once-" + itoa(s.seq))
	d := time.Until(t)
	if d < 0 {
		d = 0
	}
	timer := time.AfterFunc(d, func() {
		s.mu.Lock()
		delete(s.oneShots, id)
		s.mu.Unlock()
		fn()
	})
	s.oneShots[id] = timer
	return id, nil
}

// Cancel 取消一次性任务或 cron 任务。
func (s *Scheduler) Cancel(id JobID) {
	if raw, ok := strings.CutPrefix(string(id), "cron-"); ok {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err == nil {
			s.cron.Remove(cron.EntryID(n))
		}
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.oneShots[id]; ok {
		t.Stop()
		delete(s.oneShots, id)
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
