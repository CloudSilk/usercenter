import { useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { Trash2, MessageSquare } from "lucide-react"

import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

interface ConversationSession {
  id: string
  tenantID: string
  principalID: string
  modelAlias: string
  title: string
  messageCount: number
  totalTokens: number
  createdAt?: string
  updatedAt?: string
}

interface ConversationMessage {
  id: string
  sessionID: string
  role: string
  content: string
  tokens: number
  modelName: string
  createdAt?: string
}

interface ListResponse {
  data: ConversationSession[]
  records: number
}

function fmtTime(v?: string): string {
  if (!v) return "-"
  const n = Number(v)
  const d = !Number.isNaN(n) && String(v).length >= 10 ? new Date(n * 1000) : new Date(v)
  const t = d.getTime()
  return Number.isNaN(t) ? String(v) : d.toLocaleString()
}

export default function Conversations() {
  const queryClient = useQueryClient()
  const [principalID, setPrincipalID] = useState("")
  const [pageIndex, setPageIndex] = useState(1)
  const [selectedSession, setSelectedSession] = useState<string | null>(null)
  const pageSize = 20

  const { data, isLoading } = useQuery<ListResponse>({
    queryKey: ["conversations", principalID, pageIndex],
    queryFn: () =>
      api.get<ListResponse>("/admin/api/conversations", {
        principalID: principalID || undefined,
        pageIndex,
        pageSize,
      }),
  })

  const { data: msgData } = useQuery<{ data: ConversationMessage[] }>({
    queryKey: ["conversation-messages", selectedSession],
    queryFn: () => api.get(`/admin/api/conversations/${selectedSession}/messages`),
    enabled: !!selectedSession,
  })

  const sessions = data?.data ?? []
  const total = data?.records ?? 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  async function deleteSession(id: string) {
    if (!confirm("确认删除此会话及其所有消息？")) return
    try {
      await api.del(`/admin/api/conversations/${id}`)
      queryClient.invalidateQueries({ queryKey: ["conversations"] })
    } catch (e: any) {
      alert(e.message || "删除失败")
    }
  }

  return (
    <div className="space-y-4 p-4 md:p-6">
      <h1 className="text-2xl font-bold">对话会话</h1>
      <p className="text-sm text-muted-foreground">
        AI 网关的多轮对话记录。通过 session_id 自动维护消息历史。
      </p>

      <div className="flex items-end gap-2">
        <Input
          className="h-8 w-64"
          placeholder="按用户 ID 过滤"
          value={principalID}
          onChange={(e) => setPrincipalID(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") setPageIndex(1)
          }}
        />
        <Button size="sm" onClick={() => setPageIndex(1)}>
          查询
        </Button>
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>标题</TableHead>
              <TableHead>模型</TableHead>
              <TableHead>用户</TableHead>
              <TableHead>消息数</TableHead>
              <TableHead>Tokens</TableHead>
              <TableHead>更新时间</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={7} className="py-8 text-center text-muted-foreground">
                  加载中…
                </TableCell>
              </TableRow>
            ) : sessions.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className="py-8 text-center text-muted-foreground">
                  暂无会话
                </TableCell>
              </TableRow>
            ) : (
              sessions.map((s) => (
                <TableRow key={s.id}>
                  <TableCell className="font-medium">{s.title || "(未命名)"}</TableCell>
                  <TableCell>
                    <Badge variant="outline">{s.modelAlias}</Badge>
                  </TableCell>
                  <TableCell className="max-w-[120px] truncate font-mono text-xs" title={s.principalID}>
                    {s.principalID || "-"}
                  </TableCell>
                  <TableCell>{s.messageCount}</TableCell>
                  <TableCell>{s.totalTokens}</TableCell>
                  <TableCell className="whitespace-nowrap text-muted-foreground">
                    {fmtTime(s.updatedAt)}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setSelectedSession(s.id)}
                    >
                      <MessageSquare className="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="sm" onClick={() => deleteSession(s.id)}>
                      <Trash2 className="h-4 w-4 text-destructive" />
                    </Button>
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

      {/* 消息详情对话框 */}
      <Dialog open={!!selectedSession} onOpenChange={(v) => !v && setSelectedSession(null)}>
        <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>会话消息</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            {(msgData?.data ?? []).length === 0 ? (
              <p className="py-4 text-center text-muted-foreground">暂无消息</p>
            ) : (
              (msgData?.data ?? []).map((msg) => (
                <div
                  key={msg.id}
                  className={`rounded-lg border p-3 ${
                    msg.role === "user"
                      ? "bg-primary/5"
                      : msg.role === "assistant"
                        ? "bg-muted"
                        : "bg-muted/30"
                  }`}
                >
                  <div className="mb-1 flex items-center gap-2">
                    <Badge variant="secondary">{msg.role}</Badge>
                    {msg.modelName && (
                      <span className="text-xs text-muted-foreground">{msg.modelName}</span>
                    )}
                    <span className="text-xs text-muted-foreground">{msg.tokens} tokens</span>
                  </div>
                  <p className="whitespace-pre-wrap text-sm">{msg.content}</p>
                </div>
              ))
            )}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
