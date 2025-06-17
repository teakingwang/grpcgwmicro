package db

import (
	"github.com/teakingwang/grpcgwmicro/internal/user/model"
	"gorm.io/gorm"
)

func MigrateDB(db *gorm.DB) {
	err := db.AutoMigrate(&model.User{})
	if err != nil {
		panic(err)
	}
}
