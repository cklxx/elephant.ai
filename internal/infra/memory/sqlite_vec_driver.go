//go:build cgo
// +build cgo

package memory

import (
	"database/sql"
	"fmt"
	"reflect"
	"sync"
	"unsafe"

	sqlitevec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	sqlite3 "github.com/mattn/go-sqlite3"
)

const sqliteVecDriverName = "alex_sqlite3_vec"

var (
	sqliteVecDriverOnce sync.Once
	sqliteConnDBOnce    sync.Once
	sqliteConnDBOffset  uintptr
	sqliteConnDBKind    reflect.Kind
	sqliteConnDBErr     error
)

func ensureSQLiteVecDriverRegistered() {
	sqliteVecDriverOnce.Do(func() {
		// On Darwin, sqlite-vec-go-bindings Auto/Cancel are intentionally no-ops.
		// Always initialize sqlite-vec in this connection hook for each new DB connection.
		sql.Register(sqliteVecDriverName, &sqlite3.SQLiteDriver{
			ConnectHook: func(conn *sqlite3.SQLiteConn) error {
				handle, err := sqliteConnHandle(conn)
				if err != nil {
					return err
				}
				if err := sqlitevec.InitDBHandle(handle); err != nil {
					return fmt.Errorf("init sqlite-vec on sqlite connection: %w", err)
				}
				return nil
			},
		})
	})
}

func sqliteConnHandle(conn *sqlite3.SQLiteConn) (uintptr, error) {
	if conn == nil {
		return 0, fmt.Errorf("sqlite connection is nil")
	}
	offset, err := sqliteConnDBPtrOffset()
	if err != nil {
		return 0, err
	}
	dbPtrAddr := unsafe.Add(unsafe.Pointer(conn), offset)
	dbHandle := *(*unsafe.Pointer)(dbPtrAddr)
	if dbHandle == nil {
		return 0, fmt.Errorf("sqlite db handle is nil")
	}
	return uintptr(dbHandle), nil
}

func sqliteConnDBPtrOffset() (uintptr, error) {
	sqliteConnDBOnce.Do(func() {
		connType := reflect.TypeOf(sqlite3.SQLiteConn{})
		field, ok := connType.FieldByName("db")
		if !ok {
			sqliteConnDBErr = fmt.Errorf("sqlite3.SQLiteConn missing db field")
			return
		}
		kind := field.Type.Kind()
		if kind != reflect.Pointer && kind != reflect.UnsafePointer {
			sqliteConnDBErr = fmt.Errorf("sqlite3.SQLiteConn.db field has unsupported kind %s", field.Type.String())
			return
		}
		if field.Type.Size() != unsafe.Sizeof(unsafe.Pointer(nil)) {
			sqliteConnDBErr = fmt.Errorf("sqlite3.SQLiteConn.db field has unexpected size %d", field.Type.Size())
			return
		}
		sqliteConnDBKind = kind
		sqliteConnDBOffset = field.Offset
	})
	if sqliteConnDBErr == nil && sqliteConnDBKind == reflect.Invalid {
		sqliteConnDBErr = fmt.Errorf("sqlite3.SQLiteConn.db field metadata not initialized")
	}
	return sqliteConnDBOffset, sqliteConnDBErr
}
