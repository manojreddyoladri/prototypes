package index

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"dynamodb-on-mysql/internal/catalog"
	ddberrors "dynamodb-on-mysql/internal/errors"
	"dynamodb-on-mysql/internal/table"
	"dynamodb-on-mysql/internal/types"
)

func SyncIndexes(
	ctx context.Context,
	tx *sql.Tx,
	db *sql.DB,
	tableName string,
	oldItem types.Item,
	newItem types.Item,
) error {
	baseMeta, err := catalog.GetTable(db, tableName)
	if err != nil {
		return err
	}

	indexes, err := catalog.GetIndexes(db, tableName)
	if err != nil {
		return err
	}

	for _, idx := range indexes {
		var syncErr error
		switch idx.IndexType {
		case "LSI":
			syncErr = syncLSI(ctx, tx, idx, baseMeta, oldItem, newItem)
		case "GSI":
			syncErr = syncGSI(ctx, tx, idx, baseMeta, oldItem, newItem)
		}
		if syncErr != nil {
			return syncErr
		}
	}
	return nil
}

func CreateGSI(ctx context.Context, db *sql.DB, tableName string, def types.IndexDef) error {
	if _, err := catalog.GetTable(db, tableName); err != nil {
		return err
	}

	idxMeta := catalog.IndexMeta{
		TableName:       tableName,
		IndexName:       def.IndexName,
		IndexType:       "GSI",
		PKName:          def.PKName,
		SKName:          def.SKName,
		ProjectionType:  catalog.DefaultProjectionType(def.ProjectionType),
		ProjectionAttrs: def.ProjectionAttrs,
	}

	if err := table.CreateGSITable(ctx, db, idxMeta); err != nil {
		return err
	}
	if err := catalog.InsertIndex(db, idxMeta); err != nil {
		return err
	}

	baseMeta, err := catalog.GetTable(db, tableName)
	if err != nil {
		return err
	}

	const batchSize = 100
	var lastPK, lastSK string
	for {
		items, err := scanBaseTable(ctx, db, baseMeta, batchSize, lastPK, lastSK)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			break
		}

		for _, item := range items {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			if err := syncGSI(ctx, tx, idxMeta, baseMeta, nil, item); err != nil {
				tx.Rollback()
				return err
			}
			if err := tx.Commit(); err != nil {
				return err
			}
		}

		lastItem := items[len(items)-1]
		lastPK, _ = extractString(lastItem, baseMeta.PKName)
		if baseMeta.SKName != "" {
			lastSK, _ = extractString(lastItem, baseMeta.SKName)
		}
		if len(items) < batchSize {
			break
		}
	}

	return nil
}

func scanBaseTable(ctx context.Context, db *sql.DB, meta catalog.TableMeta, limit int, startPK, startSK string) ([]types.Item, error) {
	query := fmt.Sprintf("SELECT item_data FROM %s", table.QuotedTable(meta.TableName))
	var args []any

	if startPK != "" {
		if meta.SKName != "" {
			query += " WHERE (pk > ? OR (pk = ? AND sk > ?))"
			args = append(args, startPK, startPK, startSK)
		} else {
			query += " WHERE pk > ?"
			args = append(args, startPK)
		}
	}

	if meta.SKName != "" {
		query += " ORDER BY pk, sk"
	} else {
		query += " ORDER BY pk"
	}

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, ddberrors.MapMySQL(err)
	}
	defer rows.Close()

	var items []types.Item
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item types.Item
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func syncLSI(ctx context.Context, tx *sql.Tx, meta catalog.IndexMeta, baseMeta catalog.TableMeta,
	oldItem types.Item, newItem types.Item) error {
	shadow := table.LsiTableName(meta.TableName, meta.IndexName)

	if oldItem != nil {
		basePK, _ := extractString(oldItem, baseMeta.PKName)
		baseSK, _ := extractString(oldItem, baseMeta.SKName)
		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf("DELETE FROM %s WHERE base_pk = ? AND base_sk = ?", table.QuotedTable(shadow)),
			basePK, baseSK,
		); err != nil {
			return ddberrors.MapMySQL(err)
		}
	}

	if newItem == nil {
		return nil
	}

	lsiSK, ok := extractString(newItem, meta.SKName)
	if !ok {
		return nil
	}

	basePK, _ := extractString(newItem, baseMeta.PKName)
	baseSK, _ := extractString(newItem, baseMeta.SKName)
	projected, err := projectItem(newItem, baseMeta, meta)
	if err != nil {
		return err
	}

	var itemData interface{}
	if projected != nil {
		raw, err := json.Marshal(projected)
		if err != nil {
			return err
		}
		itemData = raw
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (pk, lsi_sk, base_pk, base_sk, item_data)
		VALUES (?, ?, ?, ?, ?)
		AS new_row ON DUPLICATE KEY UPDATE item_data = new_row.item_data`,
		table.QuotedTable(shadow)),
		basePK, lsiSK, basePK, baseSK, itemData,
	)
	return ddberrors.MapMySQL(err)
}

func syncGSI(ctx context.Context, tx *sql.Tx, meta catalog.IndexMeta, baseMeta catalog.TableMeta,
	oldItem types.Item, newItem types.Item) error {
	shadow := table.GsiTableName(meta.TableName, meta.IndexName)

	if oldItem != nil {
		basePK, _ := extractString(oldItem, baseMeta.PKName)
		baseSK, _ := extractString(oldItem, baseMeta.SKName)
		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf("DELETE FROM %s WHERE base_pk = ? AND base_sk = ?", table.QuotedTable(shadow)),
			basePK, baseSK,
		); err != nil {
			return ddberrors.MapMySQL(err)
		}
	}

	if newItem == nil {
		return nil
	}

	gsiPK, ok := extractString(newItem, meta.PKName)
	if !ok {
		return nil
	}

	var gsiSK string
	if meta.SKName != "" {
		var okSK bool
		gsiSK, okSK = extractString(newItem, meta.SKName)
		if !okSK {
			return nil
		}
	}

	basePK, _ := extractString(newItem, baseMeta.PKName)
	baseSK, _ := extractString(newItem, baseMeta.SKName)
	projected, err := projectItem(newItem, baseMeta, meta)
	if err != nil {
		return err
	}

	var itemData interface{}
	if projected != nil {
		raw, err := json.Marshal(projected)
		if err != nil {
			return err
		}
		itemData = raw
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (gsi_pk, gsi_sk, base_pk, base_sk, item_data)
		VALUES (?, ?, ?, ?, ?)
		AS new_row ON DUPLICATE KEY UPDATE item_data = new_row.item_data`,
		table.QuotedTable(shadow)),
		gsiPK, gsiSK, basePK, baseSK, itemData,
	)
	return ddberrors.MapMySQL(err)
}

func projectItem(item types.Item, baseMeta catalog.TableMeta, idx catalog.IndexMeta) (types.Item, error) {
	switch idx.ProjectionType {
	case "KEYS_ONLY":
		return nil, nil
	case "INCLUDE":
		out := make(types.Item)
		if v, ok := item[baseMeta.PKName]; ok {
			out[baseMeta.PKName] = v
		}
		if baseMeta.SKName != "" {
			if v, ok := item[baseMeta.SKName]; ok {
				out[baseMeta.SKName] = v
			}
		}
		for _, attr := range idx.ProjectionAttrs {
			if v, ok := item[attr]; ok {
				out[attr] = v
			}
		}
		return out, nil
	default:
		return item, nil
	}
}

func extractString(item types.Item, attr string) (string, bool) {
	if attr == "" {
		return "", false
	}
	v, ok := item[attr]
	if !ok {
		return "", false
	}
	switch val := v.(type) {
	case string:
		return val, true
	case json.Number:
		return val.String(), true
	default:
		return fmt.Sprint(val), true
	}
}
