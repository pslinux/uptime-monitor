//go:build !cgo

package db

import "fmt"

// newGodrorDriver 在未启用 CGO 的编译环境下不可用, 返回明确错误。
func newGodrorDriver(_ DriverConfig) (DBDriver, error) {
	return nil, fmt.Errorf("godror 驱动需要 CGO 编译环境(Oracle Instant Client), 请使用 build.sh 中 DRIVER=godror 或改用 sqlplus 驱动")
}
