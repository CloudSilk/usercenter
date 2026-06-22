import { useEffect, useRef, useState } from "react"
import { Activity, ExternalLink, Play, Square } from "lucide-react"

import { getToken } from "@/lib/api"
import type { AuditLog } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

const PRINCIPAL_KINDS = ["用户", "Agent", "服务"]
const MAX_ITEMS = 200

function fmtTime(v?: string): string {
  if (!v) return "-"
  const n = Number(v)
  const d = !Number.isNaN(n) && String(v).length >= 10 ? new Date(n * 1000) : new Date(v)
  const t = d.getTime()
  return Number.isNaN(t) ? String(v) : d.toLocaleString()
}

export default function LiveAudit() {
  const [connected, setConnected] = useState(false)
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [error, setError] = useState<string>("")
  const esRef = useRef<EventSource | null>(null)

  function connect() {
    const token = getToken()
    if (!token) {
      setError("未登录")
      return
    }
    setError("")
    // Close any existing connection first
    esRef.current?.close()

    const url = `/admin/api/audit/stream?access_token=${encodeURIComponent(token)}`
    const es = new EventSource(url)
    esRef.current = es

    es.onopen = () => setConnected(true)
    es.onmessage = (e) => {
      try {
        const log = JSON.parse(e.data) as AuditLog
        setLogs((prev) => [log, ...prev].slice(0, MAX_ITEMS))
      } catch {
        /* ignore malformed lines */
      }
    }
    es.onerror = () => {
      setConnected(false)
      setError("连接已断开（浏览器会自动重试，或点击重新连接）")
    }
  }

  function disconnect() {
    esRef.current?.close()
    esRef.current = null
    setConnected(false)
  }

  useEffect(() => {
    return () => {
      esRef.current?.close()
      esRef.current = null
    }
  }, [])

  return (
    <div className="space-y-4 p-4 md:p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-bold">实时审计监控</h1>
        <div className="flex items-center gap-2">
          <Badge variant={connected ? "default" : "secondary"}>
            <Activity className={connected ? "animate-pulse" : ""} />
            {connected ? "已连接" : "未连接"}
          </Badge>
          {connected ? (
            <Button variant="outline" size="sm" onClick={disconnect}>
              <Square /> 断开
            </Button>
          ) : (
            <Button size="sm" onClick={connect}>
              <Play /> 连接
            </Button>
          )}
          <a href="/metrics" target="_blank" rel="noreferrer">
            <Button variant="ghost" size="sm">
              <ExternalLink /> Metrics
            </Button>
          </a>
        </div>
      </div>

      {error && (
        <div className="rounded-md border border-destructive/50 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      )}

      <div className="text-sm text-muted-foreground">
        最新 {logs.length} 条（最多保留 {MAX_ITEMS} 条，新事件置顶）
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
            {logs.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className="py-8 text-center text-muted-foreground">
                  {connected ? "等待事件…" : "点击「连接」开始监听"}
                </TableCell>
              </TableRow>
            ) : (
              logs.map((log, idx) => (
                <TableRow key={log.id || `${log.createdAt}-${idx}`}>
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
    </div>
  )
}
