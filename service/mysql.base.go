package service

import (
	"errors"
	"github.com/go-sql-driver/mysql"
	"log/slog"
)

type MySQLErrorKind string

const (
	MySQLErrorNone          MySQLErrorKind = ""
	MySQLErrorDuplicate     MySQLErrorKind = "Duplicate"
	MySQLErrorDataTooLong   MySQLErrorKind = "DataTooLong"
	MySQLErrorNotNull       MySQLErrorKind = "NotNull"
	MySQLErrorForeignKeyRef MySQLErrorKind = "ForeignKeyRef"
	MySQLErrorForeignKeyDel MySQLErrorKind = "ForeignKeyDel"
	MySQLErrorOther         MySQLErrorKind = "Other"
)

func ClassifierMySQLError(err error) MySQLErrorKind {
	if err == nil {
		return MySQLErrorNone
	}
	slog.Error("SQL EXEC ERROR", "ClassifierMySQLError", err)
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		switch mysqlErr.Number {
		case 1062:
			return MySQLErrorDuplicate
		case 1406:
			return MySQLErrorDataTooLong
		case 1048:
			return MySQLErrorNotNull
		case 1452:
			return MySQLErrorForeignKeyRef
		case 1451:
			return MySQLErrorForeignKeyDel
		default:
			return MySQLErrorOther
		}
	}

	return MySQLErrorOther
}

func (k MySQLErrorKind) Label() string {
	switch k {
	case MySQLErrorDuplicate:
		return "存在重复记录"
	case MySQLErrorDataTooLong:
		return "字段长度超出限制"
	case MySQLErrorNotNull:
		return "字段不能为空"
	case MySQLErrorForeignKeyRef:
		return "外键约束错误，关联记录不存在"
	case MySQLErrorForeignKeyDel:
		return "外键约束错误，无法删除被引用的记录"
	case MySQLErrorOther:
		return "其他数据错误"
	default:
		return ""
	}
}
