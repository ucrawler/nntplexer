package main

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sync"
	"time"
)

type User struct {
	Name      string `gorm:"size:32;primaryKey"`
	Pass      string `gorm:"size:64"`
	MaxConns  uint16 `gorm:"not null;default:0"`
	IpSharing bool	 `gorm:"not null;default:0"`
	RxBytes   uint64 `gorm:"not null;default:0"`
}

type Backend struct {
	Name           string `gorm:"size:32;primaryKey"`
	User           string
	Pass           string
	Host           string
	Port           uint16
	UseTLS         bool
	Retention      uint16
	Priority       uint16
	MaxConns       uint16
	MaxFails       uint16
	FailTimeout    uint16
	ConnectTimeout uint32
	Enabled        bool
	Node           uint32
}

type Article struct {
	MessageId string `gorm:"size:250;primaryKey"`
	Date      time.Time
	CreatedAt time.Time `gorm:"index"`
}

type UserRepository struct {
	sync.RWMutex
	db    *gorm.DB
	users map[string]User
}

func (ur *UserRepository) Refresh() {
	var users []User
	result := ur.db.Find(&users)
	ur.Lock()
	defer ur.Unlock()

	ur.users = make(map[string]User, result.RowsAffected)
	for _, user := range users {
		ur.users[user.Name] = user
	}
}

func (ur *UserRepository) Get(user string) User {
	ur.RLock()
	defer ur.RUnlock()
	return ur.users[user]
}

func (ur *UserRepository) Stats(user string, rx int64, tx int64) {
	ur.db.Exec("UPDATE users SET rx_bytes = rx_bytes + ? WHERE name = ?", rx, user)
}

type BackendRepository struct {
	sync.RWMutex
	db       *gorm.DB
	backends []Backend
}

func (br *BackendRepository) Refresh() {
	var backends []Backend
	result := br.db.Order("priority").Where(&Backend{Enabled: true}).Where(&Backend{Node: 2}).Find(&backends)
	br.Lock()
	defer br.Unlock()

	br.backends = make([]Backend, result.RowsAffected)
	for index, backend := range backends {
		br.backends[index] = backend
	}
}

func (br *BackendRepository) Get() []Backend {
	br.RLock()
	defer br.RUnlock()

	return br.backends
}

func (br *BackendRepository) Stats(name string, tx int64, rx int64) {
	// TODO
}

type ArticleRepository struct {
	db *gorm.DB
}

// Cleanup removes stale articles from cache
func (ar *ArticleRepository) Cleanup(ttl int) {
	// time when article is marked as stale and removed
	staleAt := time.Now().AddDate(0, 0, -ttl)

	ar.db.Where("created_at < ?", staleAt).Delete(&Article{})
}

func (ar *ArticleRepository) Get(id string) *Article {
	var article Article
	_ = ar.db.FirstOrInit(&article, id)
	return &article
}

func (ar *ArticleRepository) Create(id string, ts time.Time) error {
	now := time.Now()

	result := ar.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "message_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"created_at": now}),
	}).Create(&Article{
		MessageId: id,
		Date:      ts,
		CreatedAt: now,
	})

	return result.Error
}
