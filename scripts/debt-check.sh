#!/bin/bash
# debt-check.sh — REDESIGN §4 CI 闸门(阶段3:阻断模式)
set -e
cd "$(dirname "$0")/.."
echo "=== REDESIGN §4 debt-check (Phase 3 blocking) ==="
PASS=true

# Gate 1: FromCurrentUser 生产代码引用 = 0(不含定义本身/test/doc/deprecated)
count1=$(grep -rnE 'FromCurrentUser' --include='*.go' internal/ model/ http/ provider/ utils/ 2>/dev/null | grep -v '_test.go' | grep -v 'doc.go' | grep -v 'func FromCurrentUser' | grep -v 'Deprecated' | grep -v '// ' | wc -l)
echo "Gate 1: FromCurrentUser production calls = $count1 (target: 0)"
if [ "$count1" -gt 0 ]; then PASS=false; fi

# Gate 2: AuthenticatePrincipal 存在且返回 Principal
count2=$(grep -c 'func AuthenticatePrincipal.*principal.Principal' model/auth.go 2>/dev/null || echo 0)
echo "Gate 2: AuthenticatePrincipal returns Principal = $count2 (target: 1)"
if [ "$count2" -lt 1 ]; then PASS=false; fi

# Gate 3: EncodeTokenFromPrincipal 存在
count3=$(grep -c 'func EncodeTokenFromPrincipal' internal/auth/token/token.go 2>/dev/null || echo 0)
echo "Gate 3: EncodeTokenFromPrincipal exists = $count3 (target: 1)"
if [ "$count3" -lt 1 ]; then PASS=false; fi

if $PASS; then
    echo "=== ALL GATES PASSED ==="
    exit 0
else
    echo "=== GATES FAILED — merge blocked ==="
    exit 1
fi
