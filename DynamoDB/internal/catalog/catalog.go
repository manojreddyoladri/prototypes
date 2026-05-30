package catalog

import (
	"database/sql"
	"encoding/json"
	"strings"
	"sync"

	ddberrors "dynamodb-on-mysql/internal/errors"
	ddbsql "dynamodb-on-mysql/sql"
	"dynamodb-on-mysql/internal/types"
)

// TableMeta holds the metadata for one DynamoDB table.
type TableMeta struct {
	TableName string
	PKName    string
	SKName    string // empty if no sort key
}

// IndexMeta holds the metadata for one LSI or GSI.
type IndexMeta struct {
	TableName       string
	IndexName       string
	IndexType       string // "LSI" or "GSI"
	PKName          string
	SKName          string
	ProjectionType  string
	ProjectionAttrs []string
}

var (
	tableCache  sync.Map // tableName -> TableMeta
	indexCache  sync.Map // tableName -> []IndexMeta
)

func Bootstrap(db *sql.DB) error {
	_, err := db.Exec(ddbsql.BootstrapSQL)
	return ddberrors.MapMySQL(err)
}

func InvalidateCache(tableName string) {
	tableCache.Delete(tableName)
	indexCache.Delete(tableName)
}

func GetTable(db *sql.DB, tableName string) (TableMeta, error) {
	if cached, ok := tableCache.Load(tableName); ok {
		return cached.(TableMeta), nil
	}

	var meta TableMeta
	var skName sql.NullString
	err := db.QueryRow(
		`SELECT table_name, pk_name, sk_name FROM _ddb_tables WHERE table_name = ?`,
		tableName,
	).Scan(&meta.TableName, &meta.PKName, &skName)
	if err == sql.ErrNoRows {
		return TableMeta{}, types.ErrTableNotFound
	}
	if err != nil {
		return TableMeta{}, ddberrors.MapMySQL(err)
	}
	if skName.Valid {
		meta.SKName = skName.String
	}

	tableCache.Store(tableName, meta)
	return meta, nil
}

func GetIndexes(db *sql.DB, tableName string) ([]IndexMeta, error) {
	if cached, ok := indexCache.Load(tableName); ok {
		return cached.([]IndexMeta), nil
	}

	rows, err := db.Query(`
		SELECT table_name, index_name, index_type, pk_name, sk_name, projection_type, projection_attrs
		FROM _ddb_indexes WHERE table_name = ?`, tableName)
	if err != nil {
		return nil, ddberrors.MapMySQL(err)
	}
	defer rows.Close()

	var indexes []IndexMeta
	for rows.Next() {
		var meta IndexMeta
		var skName, projAttrs sql.NullString
		if err := rows.Scan(
			&meta.TableName, &meta.IndexName, &meta.IndexType,
			&meta.PKName, &skName, &meta.ProjectionType, &projAttrs,
		); err != nil {
			return nil, err
		}
		if skName.Valid {
			meta.SKName = skName.String
		}
		if projAttrs.Valid && projAttrs.String != "" {
			_ = json.Unmarshal([]byte(projAttrs.String), &meta.ProjectionAttrs)
		}
		indexes = append(indexes, meta)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	indexCache.Store(tableName, indexes)
	return indexes, nil
}

func GetIndex(db *sql.DB, tableName, indexName string) (IndexMeta, error) {
	indexes, err := GetIndexes(db, tableName)
	if err != nil {
		return IndexMeta{}, err
	}
	for _, idx := range indexes {
		if idx.IndexName == indexName {
			return idx, nil
		}
	}
	return IndexMeta{}, types.ErrTableNotFound
}

func InsertTable(db *sql.DB, meta TableMeta) error {
	var sk interface{}
	if meta.SKName != "" {
		sk = meta.SKName
	}
	_, err := db.Exec(
		`INSERT INTO _ddb_tables (table_name, pk_name, sk_name) VALUES (?, ?, ?)`,
		meta.TableName, meta.PKName, sk,
	)
	if err != nil {
		return ddberrors.MapMySQL(err)
	}
	InvalidateCache(meta.TableName)
	return nil
}

func DeleteTableMeta(db *sql.DB, tableName string) error {
	if _, err := db.Exec(`DELETE FROM _ddb_indexes WHERE table_name = ?`, tableName); err != nil {
		return ddberrors.MapMySQL(err)
	}
	if _, err := db.Exec(`DELETE FROM _ddb_tables WHERE table_name = ?`, tableName); err != nil {
		return ddberrors.MapMySQL(err)
	}
	InvalidateCache(tableName)
	return nil
}

func InsertIndex(db *sql.DB, meta IndexMeta) error {
	projType := meta.ProjectionType
	if projType == "" {
		projType = "ALL"
	}
	var sk interface{}
	if meta.SKName != "" {
		sk = meta.SKName
	}
	var attrsJSON interface{}
	if len(meta.ProjectionAttrs) > 0 {
		b, err := json.Marshal(meta.ProjectionAttrs)
		if err != nil {
			return err
		}
		attrsJSON = string(b)
	}
	_, err := db.Exec(`
		INSERT INTO _ddb_indexes (table_name, index_name, index_type, pk_name, sk_name, projection_type, projection_attrs)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		meta.TableName, meta.IndexName, meta.IndexType,
		meta.PKName, sk, projType, attrsJSON,
	)
	if err != nil {
		return ddberrors.MapMySQL(err)
	}
	InvalidateCache(meta.TableName)
	return nil
}

func EncodeProjectionAttrs(attrs []string) string {
	if len(attrs) == 0 {
		return ""
	}
	b, _ := json.Marshal(attrs)
	return string(b)
}

func DefaultProjectionType(t string) string {
	if t == "" {
		return "ALL"
	}
	return strings.ToUpper(t)
}
