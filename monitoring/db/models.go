package db

import (
	"time"

	"gorm.io/gorm"
)

// Metric represents a performance metric in the database
type Metric struct {
	IP        string    `gorm:"type:varchar(255);primaryKey"`
	Value     int       `gorm:"not null"`
	Timestamp time.Time `gorm:"primaryKey;not null"`
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
func (d *Database) CreateMetric(ip string, value int) error {
	metric := Metric{
		IP:        ip,
		Value:     value,
		Timestamp: time.Now().UTC(),
	}
	return d.db.Create(&metric).Error
}

// GetMetrics retrieves all metrics
func (d *Database) GetMetrics() ([]Metric, error) {
	var metrics []Metric
	err := d.db.Find(&metrics).Error
	return metrics, err
}

// GetMetricByIP retrieves metrics for a specific IP
func (d *Database) GetMetricByIP(ip string) ([]Metric, error) {
	var metrics []Metric
	err := d.db.Where("ip = ?", ip).Find(&metrics).Error
	return metrics, err
}
