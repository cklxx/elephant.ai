package vec

// #cgo CFLAGS: -DSQLITE_CORE
// #cgo linux LDFLAGS: -lm
// #include "sqlite-vec.h"
// static int sqlite_vec_init_db(sqlite3 *db) {
//   return sqlite3_vec_init(db, 0, 0);
// }
//
import "C"
import (
	"bytes"
	"encoding/binary"
	"fmt"
	"unsafe"
)

// InitDBHandle registers sqlite-vec on a specific SQLite connection handle.
//
// The dbHandle value must be a valid sqlite3* pointer represented as uintptr.
func InitDBHandle(dbHandle uintptr) error {
	if dbHandle == 0 {
		return fmt.Errorf("sqlite db handle is nil")
	}
	rc := C.sqlite_vec_init_db((*C.sqlite3)(unsafe.Pointer(dbHandle)))
	if rc != C.SQLITE_OK {
		return fmt.Errorf("sqlite-vec init failed: rc=%d", int(rc))
	}
	return nil
}

// Serializes a float32 list into a vector BLOB that sqlite-vec accepts.
func SerializeFloat32(vector []float32) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, vector)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
