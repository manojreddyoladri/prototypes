package item

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"dynamodb-on-mysql/internal/catalog"
	ddberrors "dynamodb-on-mysql/internal/errors"
	"dynamodb-on-mysql/internal/index"
	"dynamodb-on-mysql/internal/table"
	"dynamodb-on-mysql/internal/types"
	"dynamodb-on-mysql/internal/validate"
)

func PutItem(ctx context.Context, db *sql.DB, tableName string, item types.Item) error {
	if err := validate.TableName(tableName); err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	meta, err := catalog.GetTable(db, tableName)
	if err != nil {
		return err
	}

	oldItem, err := selectForUpdate(ctx, tx, meta, itemKey(meta, item))
	if err != nil {
		return err
	}

	if err := upsertItem(ctx, tx, meta, item); err != nil {
		return err
	}

	if err := index.SyncIndexes(ctx, tx, db, tableName, oldItem, item); err != nil {
		return err
	}

	return tx.Commit()
}

func GetItem(ctx context.Context, db *sql.DB, tableName string, key types.Key) (types.Item, error) {
	if err := validate.TableName(tableName); err != nil {
		return nil, err
	}

	meta, err := catalog.GetTable(db, tableName)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf("SELECT item_data FROM %s WHERE pk = ?", table.QuotedTable(tableName))
	args := []any{key.PK}
	if meta.SKName != "" {
		query += " AND sk = ?"
		args = append(args, key.SK)
	}

	var raw []byte
	err = db.QueryRowContext(ctx, query, args...).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, ddberrors.MapMySQL(err)
	}

	var item types.Item
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil, err
	}
	return item, nil
}

func DeleteItem(ctx context.Context, db *sql.DB, tableName string, key types.Key) error {
	if err := validate.TableName(tableName); err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	meta, err := catalog.GetTable(db, tableName)
	if err != nil {
		return err
	}

	oldItem, err := selectForUpdateByKey(ctx, tx, meta, key)
	if err != nil {
		return err
	}
	if oldItem == nil {
		return nil
	}

	if err := deleteRow(ctx, tx, meta, key); err != nil {
		return err
	}

	if err := index.SyncIndexes(ctx, tx, db, tableName, oldItem, nil); err != nil {
		return err
	}

	return tx.Commit()
}

func UpdateItem(ctx context.Context, db *sql.DB, tableName string, key types.Key, update types.UpdateParams) error {
	if err := validate.TableName(tableName); err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	meta, err := catalog.GetTable(db, tableName)
	if err != nil {
		return err
	}

	oldItem, err := selectForUpdateByKey(ctx, tx, meta, key)
	if err != nil {
		return err
	}
	if oldItem == nil {
		return types.ErrItemNotFound
	}

	newItem := cloneItem(oldItem)
	for k, v := range update.Set {
		newItem[k] = v
	}
	for _, k := range update.Remove {
		delete(newItem, k)
	}

	if err := upsertItem(ctx, tx, meta, newItem); err != nil {
		return err
	}

	if err := index.SyncIndexes(ctx, tx, db, tableName, oldItem, newItem); err != nil {
		return err
	}

	return tx.Commit()
}

func selectForUpdate(ctx context.Context, tx *sql.Tx, meta catalog.TableMeta, key types.Key) (types.Item, error) {
	return selectForUpdateByKey(ctx, tx, meta, key)
}

func selectForUpdateByKey(ctx context.Context, tx *sql.Tx, meta catalog.TableMeta, key types.Key) (types.Item, error) {
	query := fmt.Sprintf("SELECT item_data FROM %s WHERE pk = ?", table.QuotedTable(meta.TableName))
	args := []any{key.PK}
	if meta.SKName != "" {
		query += " AND sk = ?"
		args = append(args, key.SK)
	}
	query += " FOR UPDATE"

	var raw []byte
	err := tx.QueryRowContext(ctx, query, args...).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, ddberrors.MapMySQL(err)
	}

	var item types.Item
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil, err
	}
	return item, nil
}

func upsertItem(ctx context.Context, tx *sql.Tx, meta catalog.TableMeta, item types.Item) error {
	pk, ok := extractString(item, meta.PKName)
	if !ok {
		return fmt.Errorf("missing partition key attribute %q", meta.PKName)
	}

	var sk string
	if meta.SKName != "" {
		var okSK bool
		sk, okSK = extractString(item, meta.SKName)
		if !okSK {
			return fmt.Errorf("missing sort key attribute %q", meta.SKName)
		}
	}

	raw, err := json.Marshal(item)
	if err != nil {
		return err
	}

	var query string
	var args []any
	if meta.SKName == "" {
		query = fmt.Sprintf(`
			INSERT INTO %s (pk, item_data) VALUES (?, ?)
			AS new_row ON DUPLICATE KEY UPDATE item_data = new_row.item_data, updated_at = NOW()`,
			table.QuotedTable(meta.TableName))
		args = []any{pk, raw}
	} else {
		query = fmt.Sprintf(`
			INSERT INTO %s (pk, sk, item_data) VALUES (?, ?, ?)
			AS new_row ON DUPLICATE KEY UPDATE item_data = new_row.item_data, updated_at = NOW()`,
			table.QuotedTable(meta.TableName))
		args = []any{pk, sk, raw}
	}

	_, err = tx.ExecContext(ctx, query, args...)
	return ddberrors.MapMySQL(err)
}

func deleteRow(ctx context.Context, tx *sql.Tx, meta catalog.TableMeta, key types.Key) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE pk = ?", table.QuotedTable(meta.TableName))
	args := []any{key.PK}
	if meta.SKName != "" {
		query += " AND sk = ?"
		args = append(args, key.SK)
	}
	_, err := tx.ExecContext(ctx, query, args...)
	return ddberrors.MapMySQL(err)
}

func itemKey(meta catalog.TableMeta, item types.Item) types.Key {
	pk, _ := extractString(item, meta.PKName)
	sk, _ := extractString(item, meta.SKName)
	return types.Key{PK: pk, SK: sk}
}

func extractString(item types.Item, attr string) (string, bool) {
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

func cloneItem(item types.Item) types.Item {
	out := make(types.Item, len(item))
	for k, v := range item {
		out[k] = v
	}
	return out
}
