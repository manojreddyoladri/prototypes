package query

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"dynamodb-on-mysql/internal/catalog"
	ddberrors "dynamodb-on-mysql/internal/errors"
	"dynamodb-on-mysql/internal/table"
	"dynamodb-on-mysql/internal/types"
	"dynamodb-on-mysql/internal/validate"
)

func Query(ctx context.Context, db *sql.DB, params types.QueryParams) ([]types.Item, error) {
	if err := validate.TableName(params.TableName); err != nil {
		return nil, err
	}

	meta, err := catalog.GetTable(db, params.TableName)
	if err != nil {
		return nil, err
	}

	if params.IndexName == "" {
		return queryBaseTable(ctx, db, meta, params)
	}

	idx, err := catalog.GetIndex(db, params.TableName, params.IndexName)
	if err != nil {
		return nil, err
	}

	switch idx.IndexType {
	case "LSI":
		return queryLSI(ctx, db, meta, idx, params)
	case "GSI":
		return queryGSI(ctx, db, meta, idx, params)
	default:
		return nil, types.ErrTableNotFound
	}
}

func queryBaseTable(ctx context.Context, db *sql.DB, meta catalog.TableMeta, params types.QueryParams) ([]types.Item, error) {
	return runBaseQuery(ctx, db, meta, table.QuotedTable(params.TableName), params)
}

func queryLSI(ctx context.Context, db *sql.DB, meta catalog.TableMeta, idx catalog.IndexMeta, params types.QueryParams) ([]types.Item, error) {
	shadow := table.LsiTableName(params.TableName, idx.IndexName)
	return runQuery(ctx, db, meta, idx, table.QuotedTable(shadow), "pk", "lsi_sk", params)
}

func queryGSI(ctx context.Context, db *sql.DB, meta catalog.TableMeta, idx catalog.IndexMeta, params types.QueryParams) ([]types.Item, error) {
	shadow := table.GsiTableName(params.TableName, idx.IndexName)
	return runQuery(ctx, db, meta, idx, table.QuotedTable(shadow), "gsi_pk", "gsi_sk", params)
}

func runBaseQuery(ctx context.Context, db *sql.DB, meta catalog.TableMeta, tableRef string, params types.QueryParams) ([]types.Item, error) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("SELECT item_data FROM %s WHERE pk = ?", tableRef))
	args := []any{params.PKValue}

	if meta.SKName != "" && params.SKOp != "" {
		frag, fragArgs, err := skCondition("sk", params)
		if err != nil {
			return nil, err
		}
		b.WriteString(" AND ")
		b.WriteString(frag)
		args = append(args, fragArgs...)
	}

	if meta.SKName != "" {
		order := "ASC"
		if !params.ScanForward {
			order = "DESC"
		}
		b.WriteString(fmt.Sprintf(" ORDER BY sk %s", order))
	}

	if params.Limit > 0 {
		b.WriteString(" LIMIT ?")
		args = append(args, params.Limit)
	}

	return scanItems(ctx, db, b.String(), args)
}

func runQuery(ctx context.Context, db *sql.DB, meta catalog.TableMeta, idx catalog.IndexMeta, tableRef, pkCol, skCol string, params types.QueryParams) ([]types.Item, error) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("SELECT item_data, base_pk, base_sk FROM %s WHERE %s = ?", tableRef, pkCol))
	args := []any{params.PKValue}

	if skCol != "" && params.SKOp != "" {
		frag, fragArgs, err := skCondition(skCol, params)
		if err != nil {
			return nil, err
		}
		b.WriteString(" AND ")
		b.WriteString(frag)
		args = append(args, fragArgs...)
	}

	if skCol != "" {
		order := "ASC"
		if !params.ScanForward {
			order = "DESC"
		}
		b.WriteString(fmt.Sprintf(" ORDER BY %s %s", skCol, order))
	}

	if params.Limit > 0 {
		b.WriteString(" LIMIT ?")
		args = append(args, params.Limit)
	}

	return scanShadowItems(ctx, db, meta, idx, b.String(), args)
}

func scanItems(ctx context.Context, db *sql.DB, query string, args []any) ([]types.Item, error) {
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

func scanShadowItems(ctx context.Context, db *sql.DB, meta catalog.TableMeta, idx catalog.IndexMeta, query string, args []any) ([]types.Item, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, ddberrors.MapMySQL(err)
	}
	defer rows.Close()

	var items []types.Item
	for rows.Next() {
		var raw sql.NullString
		var basePK, baseSK sql.NullString
		if err := rows.Scan(&raw, &basePK, &baseSK); err != nil {
			return nil, err
		}

		item, err := resolveProjectedItem(ctx, db, meta, idx, raw, basePK, baseSK)
		if err != nil {
			return nil, err
		}
		if item != nil {
			items = append(items, item)
		}
	}
	return items, rows.Err()
}

func skCondition(skCol string, params types.QueryParams) (string, []any, error) {
	switch params.SKOp {
	case types.OpEQ:
		return fmt.Sprintf("%s = ?", skCol), []any{params.SKValue}, nil
	case types.OpLT, types.OpLTE, types.OpGT, types.OpGTE:
		return fmt.Sprintf("%s %s ?", skCol, params.SKOp), []any{params.SKValue}, nil
	case types.OpBetween:
		return fmt.Sprintf("%s BETWEEN ? AND ?", skCol), []any{params.SKValue, params.SKValue2}, nil
	case types.OpBegins:
		return fmt.Sprintf("%s LIKE CONCAT(?, '%%')", skCol), []any{params.SKValue}, nil
	default:
		return "", nil, fmt.Errorf("unsupported sort key operator: %s", params.SKOp)
	}
}

func resolveProjectedItem(ctx context.Context, db *sql.DB, meta catalog.TableMeta, idx catalog.IndexMeta, raw sql.NullString, basePK, baseSK sql.NullString) (types.Item, error) {
	needsBaseLookup := idx.IndexName != "" && idx.ProjectionType == "KEYS_ONLY"
	if !needsBaseLookup && raw.Valid && raw.String != "" {
		var item types.Item
		if err := json.Unmarshal([]byte(raw.String), &item); err != nil {
			return nil, err
		}
		return item, nil
	}

	if idx.IndexName != "" && (idx.ProjectionType == "KEYS_ONLY" || !raw.Valid || raw.String == "") {
		pk := basePK.String
		sk := baseSK.String
		if pk == "" {
			return nil, nil
		}
		return getItemByKey(ctx, db, meta, pk, sk)
	}

	if raw.Valid && raw.String != "" {
		var item types.Item
		if err := json.Unmarshal([]byte(raw.String), &item); err != nil {
			return nil, err
		}
		return item, nil
	}
	return nil, nil
}

func getItemByKey(ctx context.Context, db *sql.DB, meta catalog.TableMeta, pk, sk string) (types.Item, error) {
	query := fmt.Sprintf("SELECT item_data FROM %s WHERE pk = ?", table.QuotedTable(meta.TableName))
	args := []any{pk}
	if meta.SKName != "" {
		query += " AND sk = ?"
		args = append(args, sk)
	}

	var raw []byte
	err := db.QueryRowContext(ctx, query, args...).Scan(&raw)
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

func Scan(ctx context.Context, db *sql.DB, params types.ScanParams) ([]types.Item, error) {
	if err := validate.TableName(params.TableName); err != nil {
		return nil, err
	}

	meta, err := catalog.GetTable(db, params.TableName)
	if err != nil {
		return nil, err
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("SELECT item_data FROM %s", table.QuotedTable(params.TableName)))

	var args []any
	if params.ExclusiveStartPK != "" {
		if meta.SKName != "" {
			b.WriteString(" WHERE (pk > ? OR (pk = ? AND sk > ?))")
			args = append(args, params.ExclusiveStartPK, params.ExclusiveStartPK, params.ExclusiveStartSK)
		} else {
			b.WriteString(" WHERE pk > ?")
			args = append(args, params.ExclusiveStartPK)
		}
	}

	if meta.SKName != "" {
		b.WriteString(" ORDER BY pk, sk")
	} else {
		b.WriteString(" ORDER BY pk")
	}

	if params.Limit > 0 {
		b.WriteString(" LIMIT ?")
		args = append(args, params.Limit)
	}

	rows, err := db.QueryContext(ctx, b.String(), args...)
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
