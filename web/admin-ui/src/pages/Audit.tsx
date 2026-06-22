import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Search } from "lucide-react"

import { api } from "@/lib/api"
import type { AuditLog } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

const PRINCIPAL_KINDS = ["用户", "Agent", "服务"]

interface AuditListResponse {
  data: AuditLog[]
  total: number
}

interface Filters {
  action: string
  userID: string
  principalKind: string
  startTime: string
  endTime: string
}

const emptyFilters: Filters = {
  action: "",
  userID: "",
  principalKind: "",
  startTime: "",
  endTime: "",
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

export default function Audit() {
  const [filters, setFilters] = useState<Filters>(emptyFilters)
  const [applied, setApplied] = useState<Filters>(emptyFilters)
  const [pageIndex, setPageIndex] = useState(1)
  const pageSize = 20

  const params = {
    pageIndex,
    pageSize,
    action: applied.action || undefined,
    userID: applied.userID || undefined,
    principalKind: applied.principalKind || undefined,
    startTime: toUnix(applied.startTime),
    endTime: toUnix(applied.endTime),
  }

  const { data, isLoading, isFetching } = useQuery<AuditListResponse>({
    queryKey: ["audit-logs", params],
    queryFn: () => api.get<AuditListResponse>("/admin/api/audit-logs", params),
  })

  const logs = data?.data ?? []
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  function apply() {
    setApplied(filters)
    setPageIndex(1)
  }

  function reset() {
    setFilters(emptyFilters)
    setApplied(emptyFilters)
    setPageIndex(1)
  }

  return (
    <div className="space-y-4 p-4 md:p-6">
      <h1 className="text-2xl font-bold">审计日志</h1>

      <div className="flex flex-wrap items-end gap-2">
        <div className="space-y-1">
          <Label className="text-xs">操作</Label>
          <Input
            className="h-8 w-40"
            placeholder="login / create / ..."
            value={filters.action}
            onChange={(e) => setFilters({ ...filters, action: e.target.value })}
          />
        </div>
        <div className="space-y-1">
          <Label className="text-xs">用户 ID</Label>
          <Input
            className="h-8 w-48"
            value={filters.userID}
            onChange={(e) => setFilters({ ...filters, userID: e.target.value })}
          />
        </div>
        <div className="space-y-1">
          <Label className="text-xs">主体类型</Label>
          <Select
            value={filters.principalKind}
            onValueChange={(v) => setFilters({ ...filters, principalKind: v })}
          >
            <SelectTrigger className="h-8 w-32">
              <SelectValue placeholder="全部" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">全部</SelectItem>
              {PRINCIPAL_KINDS.map((label, idx) => (
                <SelectItem key={idx} value={String(idx)}>
                  {label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-1">
          <Label className="text-xs">开始时间</Label>
          <Input
            type="datetime-local"
            className="h-8 w-48"
            value={filters.startTime}
            onChange={(e) => setFilters({ ...filters, startTime: e.target.value })}
          />
        </div>
        <div className="space-y-1">
          <Label className="text-xs">结束时间</Label>
          <Input
            type="datetime-local"
            className="h-8 w-48"
            value={filters.endTime}
            onChange={(e) => setFilters({ ...filters, endTime: e.target.value })}
          />
        </div>
        <Button size="sm" onClick={apply} disabled={isFetching}>
          <Search /> 查询
        </Button>
        <Button size="sm" variant="outline" onClick={reset}>
          重置
        </Button>
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>时间</TableHead>
              <TableHead>操作</TableHead>
              <TableHead>主体</TableHead>
              <TableHead>用户名</TableHead>
              <TableHead>目标 ID</TableHead>
              <TableHead>IP</TableHead>
              <TableHead>详情</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={7} className="py-8 text-center text-muted-foreground">
                  加载中…
                </TableCell>
              </TableRow>
            ) : logs.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className="py-8 text-center text-muted-foreground">
                  暂无数据
                </TableCell>
              </TableRow>
            ) : (
              logs.map((log) => (
                <TableRow key={log.id}>
                  <TableCell className="whitespace-nowrap text-muted-foreground">
                    {fmtTime(log.createdAt)}
                  </TableCell>
                  <TableCell className="font-medium">{log.action}</TableCell>
                  <TableCell>
                    <Badge variant="outline">
                      {PRINCIPAL_KINDS[log.principalKind] ?? log.principalKind}
                    </Badge>
                  </TableCell>
                  <TableCell>{log.userName || "-"}</TableCell>
                  <TableCell className="max-w-[160px] truncate font-mono text-xs" title={log.targetID}>
                    {log.targetID || "-"}
                  </TableCell>
                  <TableCell>{log.ip || "-"}</TableCell>
                  <TableCell className="max-w-md truncate text-xs text-muted-foreground" title={log.detail}>
                    {log.detail || "-"}
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
          <span>
            {pageIndex} / {totalPages}
          </span>
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
