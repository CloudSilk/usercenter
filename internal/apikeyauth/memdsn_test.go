package apikeyauth

import (
	"fmt"
	"sync/atomic"
)

var memDSNSeq atomic.Int64

// memDSN 返回进程内唯一的共享内存库 DSN。固定名称的 cache=shared 库在连接未关闭时
// 会跨用例/跨 -count 迭代保留数据,导致用例间相互污染;追加序号保证每次打开都是空库。
func memDSN(name string) string {
	return fmt.Sprintf("file:%s-%d?mode=memory&cache=shared", name, memDSNSeq.Add(1))
}
