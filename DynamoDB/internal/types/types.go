package types

import "fmt"

// Item is an arbitrary DynamoDB item — a map of attribute name to any JSON-compatible value.
type Item map[string]any

// Key identifies a single item in a table.
type Key struct {
	PK string
	SK string // empty string if table has no sort key
}

// KeyConditionOp enumerates the allowed operators on the sort key.
type KeyConditionOp string

const (
	OpEQ      KeyConditionOp = "="
	OpLT      KeyConditionOp = "<"
	OpLTE     KeyConditionOp = "<="
	OpGT      KeyConditionOp = ">"
	OpGTE     KeyConditionOp = ">="
	OpBetween KeyConditionOp = "BETWEEN"
	OpBegins  KeyConditionOp = "begins_with"
)

type QueryParams struct {
	TableName   string
	IndexName   string // empty = base table
	PKValue     string
	SKOp        KeyConditionOp // zero value = no SK condition
	SKValue     string
	SKValue2    string // used for BETWEEN
	ScanForward bool
	Limit       int // 0 = no limit
}

type ScanParams struct {
	TableName        string
	Limit            int
	ExclusiveStartPK string
	ExclusiveStartSK string
}

type CreateTableParams struct {
	TableName string
	PKName    string
	SKName    string // empty = no sort key
	LSIs      []IndexDef
	GSIs      []IndexDef
}

type IndexDef struct {
	IndexName       string
	PKName          string // GSI only; LSI uses base table PK
	SKName          string
	ProjectionType  string // "ALL", "KEYS_ONLY", "INCLUDE"
	ProjectionAttrs []string
}

type UpdateParams struct {
	Set    map[string]any // attribute name → new value
	Remove []string       // attribute names to delete
}

type DDBError struct {
	Code    string
	Message string
}

func (e *DDBError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

var (
	ErrTableNotFound            = &DDBError{Code: "ResourceNotFoundException", Message: "table not found"}
	ErrTableAlreadyExists       = &DDBError{Code: "ResourceInUseException", Message: "table already exists"}
	ErrItemNotFound             = &DDBError{Code: "ItemNotFoundException", Message: "item not found"}
	ErrConditionalCheckFailed   = &DDBError{Code: "ConditionalCheckFailedException", Message: "conditional check failed"}
	ErrInvalidTableName         = &DDBError{Code: "ValidationException", Message: "invalid table name"}
)
