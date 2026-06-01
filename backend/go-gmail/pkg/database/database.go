package database

import "github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/config"

type Service struct {
	config config.Config
}

func NewService() *Service {
	return &Service{}
}
