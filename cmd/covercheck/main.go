// Package main 支付域包的测试覆盖率门禁。
// 对白名单包逐一运行测试并解析覆盖率,低于阈值则以退出码 1 失败,
// 防止覆盖率回退。用法: go run ./cmd/covercheck (或 make cover-gate)。
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// gate 包路径 → 最低覆盖率阈值(百分比)。阈值取当前实测值向下取整,
// 只允许提升不允许回退;提升覆盖率后应同步上调阈值。
var gates = map[string]float64{
	"internal/wechatpay":  75,
	"internal/auth":       32,
	"internal/auth/token": 65,
	"internal/pricing":    85,
	"internal/audit":      90,
}

func main() {
	// 按包名排序保证输出稳定
	pkgs := make([]string, 0, len(gates))
	for pkg := range gates {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)

	tmp, err := os.MkdirTemp("", "cover-gate-")
	if err != nil {
		fatal(err)
	}
	defer os.RemoveAll(tmp)

	failed := false
	for _, pkg := range pkgs {
		profile := filepath.Join(tmp, strings.ReplaceAll(pkg, "/", "_")+".out")
		test := exec.Command("go", "test", "-count=1", "-coverprofile="+profile, "./"+pkg)
		test.Dir = "."
		if out, err := test.CombinedOutput(); err != nil {
			fmt.Printf("✗ %s: 测试失败\n%s\n", pkg, out)
			failed = true
			continue
		}
		total, err := parseTotal(profile)
		if err != nil {
			fatal(fmt.Errorf("解析 %s 覆盖率失败: %w", pkg, err))
		}
		threshold := gates[pkg]
		status := "✓"
		if total < threshold {
			status = "✗"
			failed = true
		}
		fmt.Printf("%s %-28s 覆盖率 %.1f%% (门禁 %.0f%%)\n", status, pkg, total, threshold)
	}
	if failed {
		os.Exit(1)
	}
	fmt.Println("覆盖率门禁全部通过")
}

// parseTotal 用 go tool cover -func 计算包的总覆盖率。
func parseTotal(profile string) (float64, error) {
	out, err := exec.Command("go", "tool", "cover", "-func="+profile).Output()
	if err != nil {
		return 0, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "total:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0, fmt.Errorf("unexpected total line: %q", line)
			}
			return strconv.ParseFloat(strings.TrimSuffix(fields[len(fields)-1], "%"), 64)
		}
	}
	return 0, fmt.Errorf("cover -func output missing total line")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
