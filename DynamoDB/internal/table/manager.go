package table

import (
	"context"
	"database/sql"
	"fmt"

	"dynamodb-on-mysql/internal/catalog"
	ddberrors "dynamodb-on-mysql/internal/errors"
	"dynamodb-on-mysql/internal/types"
	"dynamodb-on-mysql/internal/validate"
)

func CreateTable(ctx context.Context, db *sql.DB, params types.CreateTableParams) error {
	if err := validate.TableName(params.TableName); err != nil {
		return err
	}
	if err := validate.Identifier(params.PKName); err != nil {
		return err
	}
	if params.SKName != "" {
		if err := validate.Identifier(params.SKName); err != nil {
			return err
		}
	}

	meta := catalog.TableMeta{
		TableName: params.TableName,
		PKName:    params.PKName,
		SKName:    params.SKName,
	}

	if err := catalog.InsertTable(db, meta); err != nil {
		return err
	}

	if err := execBaseTableDDL(ctx, db, meta); err != nil {
		_ = catalog.DeleteTableMeta(db, params.TableName)
		return err
	}

	for _, def := range params.LSIs {
		if err := validate.IndexName(def.IndexName); err != nil {
			_ = deleteTableResources(ctx, db, params.TableName)
			return err
		}
		idxMeta := catalog.IndexMeta{
			TableName:       params.TableName,
			IndexName:       def.IndexName,
			IndexType:       "LSI",
			PKName:          params.PKName,
			SKName:          def.SKName,
			ProjectionType:  catalog.DefaultProjectionType(def.ProjectionType),
			ProjectionAttrs: def.ProjectionAttrs,
		}
		if err := createLSITable(ctx, db, idxMeta); err != nil {
			_ = deleteTableResources(ctx, db, params.TableName)
			return err
		}
		if err := catalog.InsertIndex(db, idxMeta); err != nil {
			_ = deleteTableResources(ctx, db, params.TableName)
			return err
		}
	}

	for _, def := range params.GSIs {
		if err := validate.IndexName(def.IndexName); err != nil {
			_ = deleteTableResources(ctx, db, params.TableName)
			return err
		}
		if err := validate.Identifier(def.PKName); err != nil {
			_ = deleteTableResources(ctx, db, params.TableName)
			return err
		}
		idxMeta := catalog.IndexMeta{
			TableName:       params.TableName,
			IndexName:       def.IndexName,
			IndexType:       "GSI",
			PKName:          def.PKName,
			SKName:          def.SKName,
			ProjectionType:  catalog.DefaultProjectionType(def.ProjectionType),
			ProjectionAttrs: def.ProjectionAttrs,
		}
		if err := CreateGSITable(ctx, db, idxMeta); err != nil {
			_ = deleteTableResources(ctx, db, params.TableName)
			return err
		}
		if err := catalog.InsertIndex(db, idxMeta); err != nil {
			_ = deleteTableResources(ctx, db, params.TableName)
			return err
		}
	}

	return nil
}

func DeleteTable(ctx context.Context, db *sql.DB, tableName string) error {
	if err := validate.TableName(tableName); err != nil {
		return err
	}

	if _, err := catalog.GetTable(db, tableName); err != nil {
		return err
	}

	return deleteTableResources(ctx, db, tableName)
}

func DescribeTable(ctx context.Context, db *sql.DB, tableName string) (catalog.TableMeta, error) {
	return catalog.GetTable(db, tableName)
}

func deleteTableResources(ctx context.Context, db *sql.DB, tableName string) error {
	indexes, err := catalog.GetIndexes(db, tableName)
	if err != nil {
		return err
	}
	for _, idx := range indexes {
		shadow := shadowTableName(tableName, idx)
		if _, err := db.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS `%s`", shadow)); err != nil {
			return ddberrors.MapMySQL(err)
		}
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS `%s`", tableName)); err != nil {
		return ddberrors.MapMySQL(err)
	}
	return catalog.DeleteTableMeta(db, tableName)
}

func execBaseTableDDL(ctx context.Context, db *sql.DB, meta catalog.TableMeta) error {
	var ddl string
	if meta.SKName == "" {
		ddl = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				pk          VARCHAR(255)    NOT NULL,
				item_data   JSON            NOT NULL,
				updated_at  DATETIME        DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
				PRIMARY KEY (pk)
			)`, quotedTable(meta.TableName))
	} else {
		ddl = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				pk          VARCHAR(255)    NOT NULL,
				sk          VARCHAR(255)    NOT NULL DEFAULT '',
				item_data   JSON            NOT NULL,
				updated_at  DATETIME        DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
				PRIMARY KEY (pk, sk)
			)`, quotedTable(meta.TableName))
	}
	_, err := db.ExecContext(ctx, ddl)
	return ddberrors.MapMySQL(err)
}

func createLSITable(ctx context.Context, db *sql.DB, meta catalog.IndexMeta) error {
	ddl := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			pk          VARCHAR(255)    NOT NULL,
			lsi_sk      VARCHAR(255)    NOT NULL DEFAULT '',
			base_pk     VARCHAR(255)    NOT NULL,
			base_sk     VARCHAR(255)    NOT NULL DEFAULT '',
			item_data   JSON,
			PRIMARY KEY (pk, lsi_sk, base_sk)
		)`, quotedTable(lsiTableName(meta.TableName, meta.IndexName)))
	_, err := db.ExecContext(ctx, ddl)
	return ddberrors.MapMySQL(err)
}

func CreateGSITable(ctx context.Context, db *sql.DB, meta catalog.IndexMeta) error {
	ddl := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			gsi_pk      VARCHAR(255)    NOT NULL,
			gsi_sk      VARCHAR(255)    NOT NULL DEFAULT '',
			base_pk     VARCHAR(255)    NOT NULL,
			base_sk     VARCHAR(255)    NOT NULL DEFAULT '',
			item_data   JSON,
			PRIMARY KEY (gsi_pk, gsi_sk, base_pk),
			INDEX idx_base (base_pk, base_sk)
		)`, quotedTable(gsiTableName(meta.TableName, meta.IndexName)))
	_, err := db.ExecContext(ctx, ddl)
	return ddberrors.MapMySQL(err)
}

func shadowTableName(tableName string, idx catalog.IndexMeta) string {
	if idx.IndexType == "LSI" {
		return lsiTableName(tableName, idx.IndexName)
	}
	return gsiTableName(tableName, idx.IndexName)
}

func lsiTableName(tableName, indexName string) string {
	return fmt.Sprintf("%s__lsi__%s", tableName, indexName)
}

func gsiTableName(tableName, indexName string) string {
	return fmt.Sprintf("%s__gsi__%s", tableName, indexName)
}

func quotedTable(name string) string {
	return "`" + name + "`"
}

// Exported helpers for other packages.
func LsiTableName(tableName, indexName string) string { return lsiTableName(tableName, indexName) }
func GsiTableName(tableName, indexName string) string { return gsiTableName(tableName, indexName) }
func QuotedTable(name string) string                  { return quotedTable(name) }
