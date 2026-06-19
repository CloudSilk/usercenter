#!/bin/bash
# debt-check.sh — REDESIGN §4 CI 闸门:监控 PrincipalAdapter 引用归零
#
# 4 条退出门(REDESIGN §4 阶段3 退出门):
#   1. PrincipalFromUser / PrincipalAdapter / fromLegacyUser 引用计数 = 0
#   2. model.Authenticate 返回类型签名中 *apipb.CurrentUser 出现 = 0
#   3. token.EncodeToken 中读取旧 struct 字段的直接访问 = 0(走 Principal.Encode)
#   4. (shadow mismatch metric 需要运行时数据,CI 阶段检查前 3 条)
#
# 使用:在 CI pipeline 中 ./scripts/debt-check.sh
# 退出码 0 = 通过,1 = 有债务未清(阻断合并)

set -e
cd "$(dirname "$0")/.."

echo "=== REDESIGN §4 debt-check ==="

# 闸门1:PrincipalAdapter 引用计数(不含 _test.go)
count1=$(grep -rnE 'PrincipalFromUser|PrincipalAdapter|fromLegacyUser|FromCurrentUser' --include='*.go' internal/ model/ http/ provider/ 2>/dev/null | grep -v '_test.go' | grep -v 'doc.go' | wc -l)
echo "Gate 1: PrincipalAdapter references = $count1 (target: 0)"
if [ "$count1" -gt 0 ]; then
    echo "  ⚠ Still has $count1 adapter references (transitional debt, not blocking yet)"
    # 阶段1-2 不阻断(过渡期);阶段3 应阻断
    # 取消下面注释启用阻断:
    # exit 1
fi

# 闸门2:model.Authenticate 返回 *apipb.CurrentUser 计数
count2=$(grep -n 'func Authenticate' model/auth.go 2>/dev/null | grep -c 'CurrentUser')
echo "Gate 2: Authenticate returns CurrentUser = $count2 (target: 0)"
if [ "$count2" -gt 0 ]; then
    echo "  ⚠ Authenticate still returns CurrentUser (will change in 阶段3)"
fi

# 闸门3:token.EncodeToken 读取 struct 直接字段
count3=$(grep -c 'user\.Id\|user\.UserName\|user\.RoleIDs' internal/auth/token/token.go 2>/dev/null || echo 0)
echo "Gate 3: EncodeToken direct field access = $count3 (target: 0)"
if [ "$count3" -gt 0 ]; then
    echo "  ⚠ EncodeToken still accesses struct fields directly (will change in 阶段3)"
fi

echo "=== debt-check complete (all gates in monitoring mode) ==="
echo "NOTE: Gates will block merges once 阶段3 begins (REDESIGN §4)"
exit 0
