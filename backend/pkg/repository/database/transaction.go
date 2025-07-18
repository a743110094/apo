// Copyright 2024 CloudDetail
// SPDX-License-Identifier: Apache-2.0

package database

import (
	core "github.com/CloudDetail/apo/backend/pkg/core"
	"github.com/CloudDetail/apo/backend/pkg/repository/database/driver"
)

func (repo *daoRepo) Transaction(ctx core.Context, funcs ...func(txCtx core.Context) error) (err error) {
	tx := repo.GetContextDB(ctx).Begin()
	defer func() {
		driver.FinishTransaction(ctx)
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	txCtx := driver.WithTransaction(ctx, tx)
	for _, f := range funcs {
		if err = f(txCtx); err != nil {
			tx.Rollback()
			return
		}
	}
	if err = tx.Commit().Error; err != nil {
		tx.Rollback()
	}
	return
}
