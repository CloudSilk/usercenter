package http

import "testing"

// testIsolator 由外部测试包(http_test)的 TestMain 注入:内部测试包无法直接导入 bootstrap
// (bootstrap 依赖本包,会形成导入环),因此借助 export_test 模式复用同一套隔离逻辑。
var testIsolator func(*testing.T)

// SetTestIsolator 仅在测试二进制中可见。
func SetTestIsolator(f func(*testing.T)) { testIsolator = f }

// isolate 为当前用例准备独立数据库,见 http_test.isolate。
func isolate(t *testing.T) {
	t.Helper()
	if testIsolator == nil {
		t.Fatal("test isolator not installed")
	}
	testIsolator(t)
}
