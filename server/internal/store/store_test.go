package store

import "testing"

type sample struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

func TestOpenAndAutoMigrate(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.AutoMigrate(&sample{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	if err := s.DB.Create(&sample{Name: "x"}).Error; err != nil {
		t.Fatalf("Create: %v", err)
	}
	var got sample
	if err := s.DB.First(&got).Error; err != nil {
		t.Fatalf("First: %v", err)
	}
	if got.Name != "x" {
		t.Fatalf("Name = %q, want x", got.Name)
	}
}
