package database

import "gmail-transactions/pkg/config"

type Service struct {
	config config.Config
}

func NewService() *Service {
	return &Service{}
}

