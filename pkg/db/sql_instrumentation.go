package db

import (
	"github.com/shrika/product-catalog-graphql-api/pkg/sqlstats"
	"gorm.io/gorm"
)

func registerSQLCounterCallbacks(db *gorm.DB) {
	counterCallback := func(tx *gorm.DB) {
		if tx == nil || tx.Statement == nil {
			return
		}
		if counter, ok := sqlstats.FromContext(tx.Statement.Context); ok {
			counter.Inc()
		}
	}

	_ = db.Callback().Query().Before("gorm:query").Register("sqlstats:gorm_query", counterCallback)
	_ = db.Callback().Create().Before("gorm:create").Register("sqlstats:gorm_create", counterCallback)
	_ = db.Callback().Update().Before("gorm:update").Register("sqlstats:gorm_update", counterCallback)
	_ = db.Callback().Delete().Before("gorm:delete").Register("sqlstats:gorm_delete", counterCallback)
	_ = db.Callback().Row().Before("gorm:row").Register("sqlstats:gorm_row", counterCallback)
	_ = db.Callback().Raw().Before("gorm:raw").Register("sqlstats:gorm_raw", counterCallback)
}
