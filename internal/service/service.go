package service

import (
	"database/sql"
)

type Service struct {
	Db               *sql.DB
	JWTSecret        string
	AvatarURLPrefix  string
	BrokerRepository *BrokerRepository
}

func New(db *sql.DB, jwtSecret string, avatarURLPrefix string) *Service {
	return &Service{
		Db:               db,
		JWTSecret:        jwtSecret,
		AvatarURLPrefix:  avatarURLPrefix,
		BrokerRepository: newBrokerRepository(),
	}
}
