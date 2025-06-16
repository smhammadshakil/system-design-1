package db

import (
	"gorm.io/gorm"
)

// Metric represents a performance metric in the database
type Metric struct {
	ID    uint   `gorm:"primaryKey"`
	Key   string `gorm:"type:varchar(255);not null"`
	Value int    `gorm:"not null"`
}

// Database represents the database connection and operations
type Database struct {
	db *gorm.DB
}

// NewDatabase creates a new database instance
func NewDatabase(db *gorm.DB) *Database {
	return &Database{db: db}
}

// InitDB initializes the database and runs migrations
func (d *Database) InitDB() error {
	// Auto migrate the schema
	return d.db.AutoMigrate(&Metric{})
}

// CreateMetric creates a new metric record
func (d *Database) CreateMetric(key string, value int) error {
	metric := Metric{
		Key:   key,
		Value: value,
	}
	return d.db.Create(&metric).Error
}

// GetMetrics retrieves all metrics
func (d *Database) GetMetrics() ([]Metric, error) {
	var metrics []Metric
	err := d.db.Find(&metrics).Error
	return metrics, err
}

// GetMetricByKey retrieves a metric by its key
func (d *Database) GetMetricByKey(key string) (Metric, error) {
	var metric Metric
	err := d.db.Where("key = ?", key).First(&metric).Error
	return metric, err
}
