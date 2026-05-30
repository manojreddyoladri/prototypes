package errors

import (
	"errors"

	"dynamodb-on-mysql/internal/types"

	"github.com/go-sql-driver/mysql"
)

func MapMySQL(err error) error {
	if err == nil {
		return nil
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		switch mysqlErr.Number {
		case 1050:
			return types.ErrTableAlreadyExists
		case 1146:
			return types.ErrTableNotFound
		}
	}
	return err
}
