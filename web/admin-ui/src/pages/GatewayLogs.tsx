import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Search } from "lucide-react"

import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

interface GatewayLog {
  id: string
  tenantID: string
  principalID: string
  requestID: string
  method: string
  modelAlias: string
  stream: boolean
  promptTokens: number
  compTokens: number
  cost: number
  latencyMs: number
  statusCode: number
  success: boolean
  errorMessage: string
  cached: boolean
  sessionID: string
  createdAt?: string
}

interface GatewayStats {
  totalRequests: number
  successRequests: number
  totalTokens: number
  totalCost: number
  avgLatencyMs: number
  cachedRequests: number
  cacheHitRate: number
}

interface ListResponse {
  data: GatewayLog[]
  records: number
}

function fmtTime(v?: string): string {
  if (!v) return "-"
  const n = Number(v)
  const d = !Number.isNaN(n) && String(v).length >= 10 ? new Date(n * 1000) : new Date(v)
  const t = d.getTime()
  return Number.isNaN(t) ? String(v) : d.toLocaleString()
}

function toUnix(dateStr: string): number | undefined {
  if (!dateStr) return undefined
  const t = Math.floor(new Date(dateStr).getTime() / 1000)
  return Number.isNaN(t) ? undefined : t
}

export default function GatewayLogs() {
  const [startTime, setStartTime] = useState("")
  const [endTime, setEndTime] = useState("")
  const [appliedRange, setAppliedRange] = useState({ start: "", end: "" })
  const [pageIndex, setPageIndex] = useState(1)
  const pageSize = 30

  const params = {
    pageIndex,
    pageSize,
    startTime: toUnix(appliedRange.start),
    endTime: toUnix(appliedRange.end),
  }

  const { data, isLoading } = useQuery<ListResponse>({
    queryKey: ["gateway-logs", params],
    queryFn: () => api.get<ListResponse>("/admin/api/gateway-logs", params),
  })

  const { data: statsData } = useQuery<{ data: GatewayStats }>({
    queryKey: ["gateway-stats", params],
    queryFn: () => api.get("/admin/api/gateway-logs/stats", params),
  })

  const logs = data?.data ?? []
  const total = data?.records ?? 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const stats = statsData?.data

  return (
    <div className="space-y-4 p-4 md:p-6">
      <h1 className="text-2xl font-bold">网关日志</h1>
      <p className="text-sm text-muted-foreground">
        每次 AI 网关调用的完整记录：token 用量、费用、延迟、缓存命中。
      </p>

      {/* 统计卡片 */}
      {stats && (
        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          <div className="rounded-lg border p-3">
            <p className="text-xs text-muted-foreground">总请求</p>
            <p className="text-xl font-bold">{stats.totalRequests}</p>
          </div>
          <div className="rounded-lg border p-3">
            <p className="text-xs text-muted-foreground">总 Tokens</p>
            <p className="text-xl font-bold">{stats.totalTokens.toLocaleString()}</p>
          </div>
          <div className="rounded-lg border p-3">
            <p className="text-xs text-muted-foreground">总费用 (USD)</p>
            <p className="text-xl font-bold">${stats.totalCost.toFixed(4)}</p>
          </div>
          <div className="rounded-lg border p-3">
            <p className="text-xs text-muted-foreground">缓存命中率</p>
            <p className="text-xl font-bold text-green-600">{stats.cacheHitRate.toFixed(1)}%</p>
          </div>
        </div>
      )}

      {/* 时间过滤 */}
      <div className="flex flex-wrap items-end gap-2">
        <div className="space-y-1">
          <Label className="text-xs">开始时间</Label>
          <Input
            type="datetime-local"
            className="h-8 w-48"
            value={startTime}
            onChange={(e) => setStartTime(e.target.value)}
          />
        </div>
        <div className="space-y-1">
          <Label className="text-xs">结束时间</Label>
          <Input
            type="datetime-local"
            className="h-8 w-48"
            value={endTime}
            onChange={(e) => setEndTime(e.target.value)}
          />
        </div>
        <Button
          size="sm"
          onClick={() => {
            setAppliedRange({ start: startTime, end: endTime })
            setPageIndex(1)
          }}
        >
          <Search /> 查询
        </Button>
      </div>

      {/* 日志表 */}
      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>时间</TableHead>
              <TableHead>端点</TableHead>
              <TableHead>模型</TableHead>
              <TableHead>Tokens (P/C)</TableHead>
              <TableHead>费用</TableHead>
              <TableHead>延迟</TableHead>
              <TableHead>缓存</TableHead>
              <TableHead>状态</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={8} className="py-8 text-center text-muted-foreground">
                  加载中…
                </TableCell>
              </TableRow>
            ) : logs.length === 0 ? (
              <TableRow>
                <TableCell colSpan={8} className="py-8 text-center text-muted-foreground">
                  暂无日志
                </TableCell>
              </TableRow>
            ) : (
              logs.map((log) => (
                <TableRow key={log.id}>
                  <TableCell className="whitespace-nowrap text-muted-foreground">
                    {fmtTime(log.createdAt)}
                  </TableCell>
                  <TableCell className="font-mono text-xs">{log.method}</TableCell>
                  <TableCell>
                    <Badge variant="outline">{log.modelAlias}</Badge>
                  </TableCell>
                  <TableCell className="text-xs">
                    {log.promptTokens} / {log.compTokens}
                  </TableCell>
                  <TableCell>${log.cost.toFixed(4)}</TableCell>
                  <TableCell>{log.latencyMs}ms</TableCell>
                  <TableCell>
                    {log.cached ? (
                      <Badge className="bg-green-100 text-green-700">HIT</Badge>
                    ) : (
                      <span className="text-muted-foreground">-</span>
                    )}
                  </TableCell>
                  <TableCell>
                    {log.success ? (
                      <Badge className="bg-green-100 text-green-700">成功</Badge>
                    ) : (
                      <Badge variant="destructive">{log.statusCode}</Badge>
                    )}
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>共 {total} 条</span>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={pageIndex <= 1}
            onClick={() => setPageIndex((p) => Math.max(1, p - 1))}
          >
            上一页
          </Button>
          <span>{pageIndex} / {totalPages}</span>
          <Button
            variant="outline"
            size="sm"
            disabled={pageIndex >= totalPages}
            onClick={() => setPageIndex((p) => Math.min(totalPages, p + 1))}
          >
            下一页
          </Button>
        </div>
      </div>
    </div>
  )
}
