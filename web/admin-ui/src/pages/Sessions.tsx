import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Ban, Eraser, Search } from "lucide-react"

import { api } from "@/lib/api"
import type { Session } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

const DEVICE_TYPES = ["Web", "Android", "iPhone", "iPad"]

function fmtTime(v?: number): string {
  if (!v) return "-"
  const t = Number(v)
  if (Number.isNaN(t)) return String(v)
  // lastActiveAt may be unix seconds or ms; normalize
  const ms = t > 1e12 ? t : t * 1000
  const d = new Date(ms)
  return Number.isNaN(d.getTime()) ? String(v) : d.toLocaleString()
}

interface SessionListResponse {
  data: Session[]
}

export default function Sessions() {
  const qc = useQueryClient()
  const [principalID, setPrincipalID] = useState("")
  const [active, setActive] = useState(true)
  const [applied, setApplied] = useState<{ principalID: string; active: boolean }>(
    { principalID: "", active: true },
  )

  const { data, isLoading, isFetching } = useQuery<SessionListResponse>({
    queryKey: ["sessions", applied],
    enabled: !!applied.principalID,
    queryFn: () =>
      api.get<SessionListResponse>("/admin/api/sessions", {
        principalID: applied.principalID,
        active: applied.active ? 1 : undefined,
      }),
  })

  const sessions = data?.data ?? []

  const revokeMutation = useMutation({
    mutationFn: (id: string) => api.del(`/admin/api/sessions/${id}`),
    onSuccess: () => {
      toast.success("会话已下线")
      qc.invalidateQueries({ queryKey: ["sessions"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const revokeAllMutation = useMutation({
    mutationFn: (vars: { principalID: string; reason: string }) =>
      api.post<{ count: number }>("/admin/api/sessions/revoke-all", vars),
    onSuccess: (d) => {
      toast.success(`已下线 ${(d as { count?: number }).count ?? ""} 个会话`)
      qc.invalidateQueries({ queryKey: ["sessions"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const cleanMutation = useMutation({
    mutationFn: () => api.post<{ count: number }>("/admin/api/sessions/clean"),
    onSuccess: (d) => {
      toast.success(`已清理 ${(d as { count?: number }).count ?? ""} 个过期会话`)
    },
    onError: (e: Error) => toast.error(e.message),
  })

  function apply() {
    if (!principalID.trim()) {
      toast.error("请输入 principalID")
      return
    }
    setApplied({ principalID: principalID.trim(), active })
  }

  function revokeAll() {
    if (!applied.principalID) {
      toast.error("请先查询会话")
      return
    }
    const reason = window.prompt("下线原因（可选）", "管理员手动下线") ?? ""
    if (reason === null) return
    if (!window.confirm(`确认下线 ${applied.principalID} 的全部会话？`)) return
    revokeAllMutation.mutate({ principalID: applied.principalID, reason })
  }

  return (
    <div className="space-y-4 p-4 md:p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-bold">会话管理</h1>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={revokeAll} disabled={revokeAllMutation.isPending}>
            <Ban /> 下线全部会话
          </Button>
          <Button variant="outline" size="sm" onClick={() => cleanMutation.mutate()} disabled={cleanMutation.isPending}>
            <Eraser /> 清理过期
          </Button>
        </div>
      </div>

      <div className="flex flex-wrap items-end gap-2">
        <div className="space-y-1">
          <Label className="text-xs">Principal ID</Label>
          <Input
            className="h-8 w-72"
            placeholder="用户/Agent ID"
            value={principalID}
            onChange={(e) => setPrincipalID(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") apply()
            }}
          />
        </div>
        <label className="flex h-8 items-center gap-2 text-sm">
          <Checkbox checked={active} onCheckedChange={(v) => setActive(v === true)} />
          仅活跃
        </label>
        <Button size="sm" onClick={apply} disabled={isFetching}>
          <Search /> 查询
        </Button>
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>设备名称</TableHead>
              <TableHead>类型</TableHead>
              <TableHead>IP</TableHead>
              <TableHead>位置</TableHead>
              <TableHead>User-Agent</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>最后活跃</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {!applied.principalID ? (
              <TableRow>
                <TableCell colSpan={8} className="py-8 text-center text-muted-foreground">
                  请输入 Principal ID 后查询
                </TableCell>
              </TableRow>
            ) : isLoading ? (
              <TableRow>
                <TableCell colSpan={8} className="py-8 text-center text-muted-foreground">
                  加载中…
                </TableCell>
              </TableRow>
            ) : sessions.length === 0 ? (
              <TableRow>
                <TableCell colSpan={8} className="py-8 text-center text-muted-foreground">
                  暂无数据
                </TableCell>
              </TableRow>
            ) : (
              sessions.map((s) => (
                <TableRow key={s.id}>
                  <TableCell className="font-medium">{s.deviceName || "-"}</TableCell>
                  <TableCell>{DEVICE_TYPES[s.deviceType] ?? s.deviceType}</TableCell>
                  <TableCell>{s.ip || "-"}</TableCell>
                  <TableCell>{s.location || "-"}</TableCell>
                  <TableCell className="max-w-xs truncate text-xs text-muted-foreground" title={s.userAgent}>
                    {s.userAgent || "-"}
                  </TableCell>
                  <TableCell>
                    {s.revoked ? (
                      <Badge variant="secondary">已下线</Badge>
                    ) : (
                      <Badge>活跃</Badge>
                    )}
                  </TableCell>
                  <TableCell className="text-muted-foreground">{fmtTime(s.lastActiveAt)}</TableCell>
                  <TableCell className="text-right">
                    {!s.revoked && (
                      <Button
                        size="sm"
                        variant="ghost"
                        title="下线"
                        onClick={() => {
                          if (window.confirm("确认下线该会话？"))
                            revokeMutation.mutate(s.id)
                        }}
                      >
                        <Ban className="text-destructive" /> 下线
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
