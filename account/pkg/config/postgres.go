package config

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type postgresConfig struct {
	connectionString string
}

func (c postgresConfig) BuildGorm() (*gorm.DB, error) {
	return gorm.Open(postgres.New(postgres.Config{
		DSN:                  c.connectionString,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
}

func NewPostgresConfig() postgresConfig {
	return postgresConfig{
		connectionString: env("DB_CONNECTION_STRING", "user=postgres password=postgres host=0.0.0.0 port=5432 dbname=postgres"),
	}
}
