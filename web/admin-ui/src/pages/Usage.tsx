import { useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import {
  Bar,
  BarChart,
  Cell,
  Legend,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts"
import { Activity, Coins, Timer } from "lucide-react"

import { api } from "@/lib/api"
import type { UsageRecord, UsageSummary } from "@/lib/types"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Card,
  CardContent,
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

const PIE_COLORS = ["#8884d8", "#82ca9d", "#ffc658", "#ff7300", "#00C49F", "#FFBB28"]

interface ModelAgg {
  totalTokens: number
  totalCost: number
}

// /admin/api/usage/by-principal 行：GORM 扫描到 map，键为 snake_case；数值可能为 number/string。
interface UsagePrincipalRow {
  principal_id?: string
  principal_kind?: number | string
  tokens?: number | string
  cost?: number | string
  count?: number | string
}

const PRINCIPAL_KIND_LABEL: Record<number, string> = { 0: "人", 1: "Agent", 2: "Service" }

function fmtCost(v: number): string {
  if (!v) return "0.00"
  return v.toFixed(4)
}

function fmtTime(v?: string): string {
  if (!v) return "-"
  const n = Number(v)
  const d = !Number.isNaN(n) && String(v).length >= 10 ? new Date(n * 1000) : new Date(v)
  const t = d.getTime()
  return Number.isNaN(t) ? String(v) : d.toLocaleString()
}

export default function Usage() {
  const today = new Date()
  const sevenAgo = new Date(today.getTime() - 7 * 24 * 3600 * 1000)
  const [startTime, setStartTime] = useState(sevenAgo.toISOString().slice(0, 10))
  const [endTime, setEndTime] = useState(today.toISOString().slice(0, 10))
  const [applied, setApplied] = useState({
    startTime: sevenAgo.toISOString().slice(0, 10),
    endTime: today.toISOString().slice(0, 10),
  })

  const rangeParams = useMemo(
    () => ({
      startTime: applied.startTime
        ? new Date(applied.startTime + "T00:00:00").toISOString()
        : undefined,
      endTime: applied.endTime
        ? new Date(applied.endTime + "T23:59:59").toISOString()
        : undefined,
    }),
    [applied],
  )

  const { data: summary, isLoading: sumLoading } = useQuery<UsageSummary>({
    queryKey: ["usage-summary", applied],
    queryFn: () =>
      api.get<UsageSummary>("/admin/api/usage/summary", {
        startTime: rangeParams.startTime,
        endTime: rangeParams.endTime,
      }),
  })

  const { data: byModelRaw } = useQuery<Record<string, ModelAgg>>({
    queryKey: ["usage-by-model", applied],
    queryFn: () =>
      api.get<Record<string, ModelAgg>>("/admin/api/usage/by-model", {
        startTime: rangeParams.startTime,
        endTime: rangeParams.endTime,
      }),
  })

  // 按主体维度聚合：直接消费后端 /usage/by-principal（DB 全量聚合，区分人/Agent/Service），
  // 取代过去仅在客户端用最近 50 条记录近似估算主体用量的做法。
  const { data: byPrincipalRaw } = useQuery<UsagePrincipalRow[]>({
    queryKey: ["usage-by-principal", applied],
    queryFn: () =>
      api.get<UsagePrincipalRow[]>("/admin/api/usage/by-principal", {
        startTime: rangeParams.startTime,
        endTime: rangeParams.endTime,
      }),
  })

  const { data: recent, isLoading: recentLoading } = useQuery<UsageRecord[]>({
    queryKey: ["usage-recent"],
    queryFn: () => api.get<UsageRecord[]>("/admin/api/usage/recent", { limit: 50 }),
  })

  const modelData = useMemo(() => {
    const obj = byModelRaw ?? {}
    return Object.entries(obj)
      .map(([name, v]) => ({ name, value: v?.totalTokens ?? 0, cost: v?.totalCost ?? 0 }))
      .filter((d) => d.value > 0)
      .sort((a, b) => b.value - a.value)
  }, [byModelRaw])

  // principal_kind: 0=人, 1=Agent, 2=Service（见后端 UsageRecord 注释）。
  const principalData = useMemo(() => {
    const toNum = (v: unknown): number => {
      if (typeof v === "number") return v
      if (typeof v === "string") return Number(v) || 0
      return 0
    }
    return (byPrincipalRaw ?? [])
      .map((r) => ({
        id: String(r.principal_id ?? "unknown"),
        kind: Number(toNum(r.principal_kind)),
        tokens: toNum(r.tokens),
        cost: toNum(r.cost),
        count: toNum(r.count),
      }))
      .filter((d) => d.tokens > 0 || d.count > 0)
      .sort((a, b) => b.tokens - a.tokens)
      .slice(0, 12)
  }, [byPrincipalRaw])

  const records = recent ?? []

  return (
    <div className="space-y-4 p-4 md:p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-bold">用量统计</h1>
        <div className="flex flex-wrap items-end gap-2">
          <div className="space-y-1">
            <Label className="text-xs">开始日期</Label>
            <Input
              type="date"
              className="h-8 w-40"
              value={startTime}
              onChange={(e) => setStartTime(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label className="text-xs">结束日期</Label>
            <Input
              type="date"
              className="h-8 w-40"
              value={endTime}
              onChange={(e) => setEndTime(e.target.value)}
            />
          </div>
          <Button
            size="sm"
            onClick={() => setApplied({ startTime, endTime })}
          >
            查询
          </Button>
        </div>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">总 Token</CardTitle>
            <Activity className="size-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {sumLoading ? "…" : (summary?.totalTokens ?? 0).toLocaleString()}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">总成本</CardTitle>
            <Coins className="size-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {sumLoading ? "…" : "$" + fmtCost(summary?.totalCost ?? 0)}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">请求数</CardTitle>
            <Timer className="size-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {sumLoading ? "…" : (summary?.requestCount ?? 0).toLocaleString()}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">按模型 Token 分布</CardTitle>
          </CardHeader>
          <CardContent>
            {modelData.length === 0 ? (
              <div className="py-10 text-center text-sm text-muted-foreground">暂无数据</div>
            ) : (
              <ResponsiveContainer width="100%" height={280}>
                <PieChart>
                  <Pie
                    data={modelData}
                    dataKey="value"
                    nameKey="name"
                    cx="50%"
                    cy="50%"
                    outerRadius={90}
                    label={(e: { name?: string }) => (e.name ?? "").slice(0, 16)}
                  >
                    {modelData.map((_, i) => (
                      <Cell key={i} fill={PIE_COLORS[i % PIE_COLORS.length]} />
                    ))}
                  </Pie>
                  <Tooltip
                    formatter={(v: unknown) => Number(v).toLocaleString() + " tokens"}
                  />
                  <Legend />
                </PieChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">按主体 Token 用量 (Top 12)</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {principalData.length === 0 ? (
              <div className="py-10 text-center text-sm text-muted-foreground">暂无数据</div>
            ) : (
              <>
                <ResponsiveContainer width="100%" height={220}>
                  <BarChart data={principalData} margin={{ left: 8, right: 16 }}>
                    <XAxis
                      dataKey="id"
                      tick={{ fontSize: 11 }}
                      interval={0}
                      angle={-20}
                      textAnchor="end"
                      height={60}
                    />
                    <YAxis tick={{ fontSize: 11 }} />
                    <Tooltip formatter={(v: unknown) => Number(v).toLocaleString() + " tokens"} />
                    <Bar dataKey="tokens" fill="#8884d8" />
                  </BarChart>
                </ResponsiveContainer>
                <div className="overflow-x-auto">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>主体</TableHead>
                        <TableHead>类型</TableHead>
                        <TableHead className="text-right">请求数</TableHead>
                        <TableHead className="text-right">Tokens</TableHead>
                        <TableHead className="text-right">成本</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {principalData.map((p) => (
                        <TableRow key={p.id}>
                          <TableCell className="max-w-[160px] truncate font-mono text-xs" title={p.id}>
                            {p.id}
                          </TableCell>
                          <TableCell>
                            <Badge variant="outline">{PRINCIPAL_KIND_LABEL[p.kind] ?? "未知"}</Badge>
                          </TableCell>
                          <TableCell className="text-right">{p.count.toLocaleString()}</TableCell>
                          <TableCell className="text-right">{p.tokens.toLocaleString()}</TableCell>
                          <TableCell className="text-right">${fmtCost(p.cost)}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              </>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Recent records */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">最近请求 (最多 50 条)</CardTitle>
        </CardHeader>
        <CardContent className="px-2">
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>模型</TableHead>
                  <TableHead>主体</TableHead>
                  <TableHead className="text-right">Prompt</TableHead>
                  <TableHead className="text-right">Completion</TableHead>
                  <TableHead className="text-right">Total</TableHead>
                  <TableHead className="text-right">Cost</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead className="text-right">Latency</TableHead>
                  <TableHead>时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {recentLoading ? (
                  <TableRow>
                    <TableCell colSpan={9} className="py-8 text-center text-muted-foreground">
                      加载中…
                    </TableCell>
                  </TableRow>
                ) : records.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={9} className="py-8 text-center text-muted-foreground">
                      暂无数据
                    </TableCell>
                  </TableRow>
                ) : (
                  records.map((r, i) => (
                    <TableRow key={r.id || i}>
                      <TableCell className="font-medium">{r.model || "-"}</TableCell>
                      <TableCell className="max-w-32 truncate" title={r.principalID}>
                        {r.principalID || "-"}
                      </TableCell>
                      <TableCell className="text-right">{r.promptTokens ?? 0}</TableCell>
                      <TableCell className="text-right">{r.completionTokens ?? 0}</TableCell>
                      <TableCell className="text-right">{r.totalTokens ?? 0}</TableCell>
                      <TableCell className="text-right">${fmtCost(r.cost ?? 0)}</TableCell>
                      <TableCell>
                        {r.success ? (
                          <Badge>成功</Badge>
                        ) : (
                          <Badge variant="destructive">失败</Badge>
                        )}
                      </TableCell>
                      <TableCell className="text-right">{(r.latencyMs ?? 0)}ms</TableCell>
                      <TableCell className="text-muted-foreground">{fmtTime(r.createdAt)}</TableCell>
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
