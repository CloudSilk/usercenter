// Package main swagger 文档过期检查:就地重新生成文档并用 git diff 检测变化。
// 用法: go run ./cmd/doccheck (或 make doc-check)。
// 文档落后于代码注解时 git diff 非空,以退出码 1 失败,提示运行 make gen-doc,
// 供 make test/CI 门禁调用,防止端点注解与文档脱节。
package main

import (
	"fmt"
	"os"
	"os/exec"
)

const genFlags = "--parseDependency --parseInternal --parseDepth 2"

func main() {
	swag := exec.Command("swag", "init",
		"--parseDependency", "--parseInternal", "--parseDepth", "2")
	if out, err := swag.CombinedOutput(); err != nil {
		fatal(fmt.Errorf("运行 swag init %s 失败(请确认已安装 swag CLI: go install github.com/swaggo/swag/cmd/swag@latest): %v\n%s",
			genFlags, err, out))
	}
	// git diff --exit-code: 无差异退出 0,有差异退出 1(文档过期)
	diff := exec.Command("git", "diff", "--exit-code", "--", "docs/")
	if out, err := diff.CombinedOutput(); err != nil {
		fmt.Println(string(out))
		fmt.Println("swagger 文档过期: docs/ 存在未提交的再生成差异(上方 diff),运行 'make gen-doc' 后一并提交")
		os.Exit(1)
	}
	fmt.Println("swagger 文档已是最新")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
