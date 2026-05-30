package validate

import (
	"regexp"

	"dynamodb-on-mysql/internal/types"
)

var (
	tableNamePattern  = regexp.MustCompile(`^[A-Za-z0-9_\-\.]{3,255}$`)
	identifierPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
)

func TableName(name string) error {
	if !tableNamePattern.MatchString(name) {
		return types.ErrInvalidTableName
	}
	return nil
}

func Identifier(name string) error {
	if !identifierPattern.MatchString(name) {
		return types.ErrInvalidTableName
	}
	return nil
}

func IndexName(name string) error {
	return Identifier(name)
}
