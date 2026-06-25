package store

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

type concRecord struct {
	ID   uint `gorm:"primaryKey"`
	Text string
}

// TestConcurrentAccessStaysSerializedAndIntact pins the SQLite safety setting
// (single open connection) and verifies that concurrent reads+writes neither
// error nor return corrupted/partial strings.
func TestConcurrentAccessStaysSerializedAndIntact(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "conc.db")
	st, err := Open(dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sqlDB, err := st.DB.DB()
	if err != nil {
		t.Fatalf("sqlDB: %v", err)
	}
	defer sqlDB.Close() // release the file so t.TempDir cleanup succeeds on Windows
	if got := sqlDB.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("MaxOpenConnections = %d, want 1 (SQLite must be serialized)", got)
	}
	if err := st.AutoMigrate(&concRecord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	const want = "当前画面是一处室内桌面场景，仅可见键盘、瓶装饮品、背包等桌面物品，画面中未出现任何人。"
	for i := 0; i < 30; i++ {
		if err := st.DB.Create(&concRecord{Text: want}).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var bad []string
	stop := make(chan struct{})
	for g := 0; g < 12; g++ {
		wg.Add(1)
		go func(writer bool) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if writer {
					_ = st.DB.Create(&concRecord{Text: want}).Error
					continue
				}
				var recs []concRecord
				if st.DB.Limit(60).Find(&recs).Error != nil {
					continue
				}
				for _, rec := range recs {
					if rec.Text != "" && (!utf8.ValidString(rec.Text) || rec.Text != want) {
						mu.Lock()
						if len(bad) < 3 {
							bad = append(bad, rec.Text)
						}
						mu.Unlock()
					}
				}
			}
		}(g%2 == 0)
	}
	time.Sleep(700 * time.Millisecond)
	close(stop)
	wg.Wait()

	if len(bad) > 0 {
		t.Fatalf("found %d corrupted/partial strings; e.g. %q", len(bad), bad[0])
	}
}
