import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { QRCodeSVG } from "qrcode.react"
import { KeyRound, ShieldCheck, Trash2 } from "lucide-react"

import { api } from "@/lib/api"
import type { AlertConfig, ExternalIdentity, MFAFactor } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

interface EnrollResult {
  secret: string
  uri: string
  account: string
}
interface RiskInputs {
  currentIP: string
  tokenIP: string
  tokenAgeHours: number
  newDevice: boolean
  failedAuths: number
}

const initialRisk: RiskInputs = {
  currentIP: "1.2.3.4",
  tokenIP: "1.2.3.4",
  tokenAgeHours: 1,
  newDevice: false,
  failedAuths: 0,
}

function computeRisk(r: RiskInputs): { score: number; decision: string } {
  let score = 0
  if (r.currentIP && r.tokenIP && r.currentIP !== r.tokenIP) score += 40
  if (r.tokenAgeHours > 24) score += 20
  if (r.tokenAgeHours > 24 * 7) score += 20
  if (r.newDevice) score += 25
  score += Math.min(40, Math.max(0, r.failedAuths) * 10)
  score = Math.min(100, score)
  const decision = score >= 70 ? "deny" : score >= 40 ? "step_up" : "allow"
  return { score, decision }
}

export default function Security() {
  const qc = useQueryClient()

  const { data: status } = useQuery<AlertConfig>({
    queryKey: ["alerts-status"],
    queryFn: () => api.get<AlertConfig>("/admin/api/alerts/status"),
  })

  const { data: factors } = useQuery<MFAFactor[]>({
    queryKey: ["mfa-factors"],
    queryFn: () => api.get<MFAFactor[]>("/admin/api/mfa/factors"),
  })

  const { data: identities } = useQuery<ExternalIdentity[]>({
    queryKey: ["identities"],
    queryFn: () => api.get<ExternalIdentity[]>("/admin/api/identities"),
  })

  // Risk calculator
  const [risk, setRisk] = useState<RiskInputs>(initialRisk)
  const riskResult = computeRisk(risk)

  // MFA enrollment
  const [enroll, setEnroll] = useState<EnrollResult | null>(null)
  const [code, setCode] = useState("")
  const [name, setName] = useState("")

  const enrollMutation = useMutation({
    mutationFn: () => api.post<EnrollResult>("/admin/api/mfa/totp/enroll", {}),
    onSuccess: (d) => {
      setEnroll(d)
      setCode("")
      setName("")
      toast.success("密钥已生成，请用验证器扫码")
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const confirmMutation = useMutation({
    mutationFn: (vars: { secret: string; code: string; name: string }) =>
      api.post("/admin/api/mfa/totp/confirm", vars),
    onSuccess: () => {
      toast.success("MFA 已绑定")
      setEnroll(null)
      setCode("")
      setName("")
      qc.invalidateQueries({ queryKey: ["mfa-factors"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const delFactorMutation = useMutation({
    mutationFn: (id: string) => api.del(`/admin/api/mfa/${id}`),
    onSuccess: () => {
      toast.success("已解绑")
      qc.invalidateQueries({ queryKey: ["mfa-factors"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const delIdentityMutation = useMutation({
    mutationFn: (id: string) => api.del(`/admin/api/identities/${id}`),
    onSuccess: () => {
      toast.success("已解绑")
      qc.invalidateQueries({ queryKey: ["identities"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  function confirmEnroll() {
    if (!enroll) return
    if (!/^\d{4,8}$/.test(code.trim())) {
      toast.error("请输入有效的验证码")
      return
    }
    confirmMutation.mutate({ secret: enroll.secret, code: code.trim(), name: name.trim() })
  }

  return (
    <div className="space-y-6 p-4 md:p-6">
      <h1 className="text-2xl font-bold">安全中心</h1>

      {/* Login protection policy */}
      <Card>
        <CardHeader>
          <CardTitle>登录保护策略</CardTitle>
          <CardDescription>来自 alerts/status 的当前阈值配置</CardDescription>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <div className="rounded-md border p-3">
            <div className="text-xs text-muted-foreground">登录失败阈值</div>
            <div className="mt-1 text-2xl font-bold">
              {status?.loginFailThreshold ?? "-"}
            </div>
          </div>
          <div className="rounded-md border p-3">
            <div className="text-xs text-muted-foreground">鉴权失败阈值</div>
            <div className="mt-1 text-2xl font-bold">
              {status?.authFailThreshold ?? "-"}
            </div>
          </div>
          <div className="rounded-md border p-3">
            <div className="text-xs text-muted-foreground">统计窗口(分钟)</div>
            <div className="mt-1 text-2xl font-bold">
              {status?.windowMinutes ?? "-"}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Risk score demo */}
      <Card>
        <CardHeader>
          <CardTitle>风险评分演示</CardTitle>
          <CardDescription>客户端演示性计算（非服务端策略）</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-6 md:grid-cols-2">
          <div className="space-y-3">
            <div className="space-y-1">
              <Label className="text-xs">当前 IP</Label>
              <Input
                className="h-8"
                value={risk.currentIP}
                onChange={(e) => setRisk({ ...risk, currentIP: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">Token 签发 IP</Label>
              <Input
                className="h-8"
                value={risk.tokenIP}
                onChange={(e) => setRisk({ ...risk, tokenIP: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">Token 年龄(小时)</Label>
              <Input
                type="number"
                className="h-8"
                value={risk.tokenAgeHours}
                onChange={(e) =>
                  setRisk({ ...risk, tokenAgeHours: Number(e.target.value) || 0 })
                }
              />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">失败鉴权次数</Label>
              <Input
                type="number"
                className="h-8"
                value={risk.failedAuths}
                onChange={(e) =>
                  setRisk({ ...risk, failedAuths: Number(e.target.value) || 0 })
                }
              />
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={risk.newDevice}
                onChange={(e) => setRisk({ ...risk, newDevice: e.target.checked })}
              />
              新设备
            </label>
          </div>
          <div className="flex flex-col items-center justify-center rounded-md border bg-muted/30 p-6">
            <div className="text-sm text-muted-foreground">风险评分</div>
            <div className="my-2 text-5xl font-bold">{riskResult.score}</div>
            <Badge
              variant={
                riskResult.decision === "allow"
                  ? "default"
                  : riskResult.decision === "step_up"
                    ? "secondary"
                    : "destructive"
              }
              className="text-sm"
            >
              {riskResult.decision === "allow"
                ? "放行 allow"
                : riskResult.decision === "step_up"
                  ? "二次验证 step_up"
                  : "拒绝 deny"}
            </Badge>
          </div>
        </CardContent>
      </Card>

      {/* MFA TOTP enrollment */}
      <Card>
        <CardHeader>
          <CardTitle>MFA / TOTP 绑定</CardTitle>
          <CardDescription>生成 TOTP 密钥并扫码绑定</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {!enroll ? (
            <Button size="sm" onClick={() => enrollMutation.mutate()} disabled={enrollMutation.isPending}>
              <KeyRound /> 生成密钥
            </Button>
          ) : (
            <div className="flex flex-col gap-6 md:flex-row">
              <div className="flex flex-col items-center gap-2">
                <div className="rounded-md border bg-white p-3">
                  <QRCodeSVG value={enroll.uri} size={200} />
                </div>
                <div className="text-xs text-muted-foreground">用 Google Authenticator / 1Password 扫码</div>
              </div>
              <div className="flex-1 space-y-3">
                <div className="space-y-1">
                  <Label className="text-xs">Secret</Label>
                  <Input className="h-8 font-mono text-xs" readOnly value={enroll.secret} />
                </div>
                <div className="space-y-1">
                  <Label className="text-xs">Account</Label>
                  <Input className="h-8" readOnly value={enroll.account} />
                </div>
                <div className="space-y-1">
                  <Label className="text-xs">设备名称</Label>
                  <Input
                    className="h-8"
                    placeholder="如：iPhone 15"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                  />
                </div>
                <div className="space-y-1">
                  <Label className="text-xs">验证码</Label>
                  <Input
                    className="h-8"
                    placeholder="6 位动态码"
                    value={code}
                    onChange={(e) => setCode(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") confirmEnroll()
                    }}
                  />
                </div>
                <div className="flex gap-2">
                  <Button size="sm" onClick={confirmEnroll} disabled={confirmMutation.isPending}>
                    <ShieldCheck /> 确认绑定
                  </Button>
                  <Button size="sm" variant="outline" onClick={() => setEnroll(null)}>
                    取消
                  </Button>
                </div>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* MFA factors */}
      <Card>
        <CardHeader>
          <CardTitle>已绑定的 MFA 因子</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>名称</TableHead>
                  <TableHead>类型</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(factors ?? []).length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={4} className="py-6 text-center text-muted-foreground">
                      暂无因子
                    </TableCell>
                  </TableRow>
                ) : (
                  (factors ?? []).map((f) => (
                    <TableRow key={f.id}>
                      <TableCell className="font-medium">{f.name || "-"}</TableCell>
                      <TableCell>{f.type}</TableCell>
                      <TableCell>
                        {f.enable ? <Badge>启用</Badge> : <Badge variant="secondary">禁用</Badge>}
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          size="icon"
                          variant="ghost"
                          title="解绑"
                          onClick={() => {
                            if (window.confirm(`确认解绑因子 ${f.name || f.id}？`))
                              delFactorMutation.mutate(f.id)
                          }}
                        >
                          <Trash2 className="text-destructive" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      {/* External identities */}
      <Card>
        <CardHeader>
          <CardTitle>外部身份绑定</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Provider</TableHead>
                  <TableHead>登录账号</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(identities ?? []).length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={3} className="py-6 text-center text-muted-foreground">
                      暂无绑定
                    </TableCell>
                  </TableRow>
                ) : (
                  (identities ?? []).map((it) => (
                    <TableRow key={it.id}>
                      <TableCell className="font-medium">{it.provider}</TableCell>
                      <TableCell>{it.providerLogin || "-"}</TableCell>
                      <TableCell className="text-right">
                        <Button
                          size="icon"
                          variant="ghost"
                          title="解绑"
                          onClick={() => {
                            if (window.confirm(`确认解绑 ${it.provider} 账号 ${it.providerLogin}？`))
                              delIdentityMutation.mutate(it.id)
                          }}
                        >
                          <Trash2 className="text-destructive" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
