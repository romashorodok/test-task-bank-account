package eventstore

import (
	"context"

	"gorm.io/gorm"
)

func WithTransaction(ctx context.Context, db *gorm.DB, f func(tx *gorm.DB) error) (err error) {
	tx := db.Begin().WithContext(ctx)

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		} else if err != nil {
			err = tx.Rollback().Error
		} else {
			err = tx.Commit().Error
		}
	}()

	err = f(tx)

	return err
}
