package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) AutoMigrate() error {
	return s.db.AutoMigrate(&Fact{})
}

func (s *Store) UpsertFacts(ctx context.Context, deviceID string, facts []string, sourceAt time.Time) (int, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return 0, ErrInvalidInput
	}

	added := 0
	for _, text := range normalizeFacts(facts) {
		fact := Fact{
			DeviceID: deviceID,
			Hash:     factHash(text),
			Text:     text,
			SourceAt: sourceAt,
		}
		result := s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "device_id"}, {Name: "hash"}},
			DoNothing: true,
		}).Create(&fact)
		if result.Error != nil {
			return added, result.Error
		}
		if result.RowsAffected > 0 {
			added++
		}
	}
	return added, nil
}

func (s *Store) ListFacts(ctx context.Context, deviceID string, limit int) ([]string, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, ErrInvalidInput
	}
	if limit <= 0 {
		limit = defaultMaxFacts
	}

	var facts []Fact
	err := s.db.WithContext(ctx).
		Where("device_id = ?", deviceID).
		Order("updated_at desc, id desc").
		Limit(limit).
		Find(&facts).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(facts))
	for _, fact := range facts {
		out = append(out, fact.Text)
	}
	return out, nil
}

func normalizeFacts(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func factHash(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}
