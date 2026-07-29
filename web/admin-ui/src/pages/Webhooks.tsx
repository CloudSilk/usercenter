import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Pencil, Plus, Trash2, Webhook } from "lucide-react"

import { api } from "@/lib/api"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
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
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

// 系统实际会触发的事件（见后端 alert.FireEvent 调用点）。
const KNOWN_EVENTS = [
  "user.created",
  "user.deleted",
  "role.updated",
  "tenant.created",
]

interface WebhookSub {
  id: string
  tenantID: string
  name: string
  url: string
  events: string // 逗号分隔
  secret: string
  enable: boolean
}

interface ListResp {
  data: WebhookSub[]
  total?: number
}

const empty: Omit<WebhookSub, "id"> = {
  tenantID: "",
  name: "",
  url: "",
  events: "",
  secret: "",
  enable: true,
}

export default function Webhooks() {
  const qc = useQueryClient()
  const { data, isLoading } = useQuery<WebhookSub[]>({
    queryKey: ["webhooks"],
    queryFn: async () => {
      const res = await api.get<ListResp>("/admin/api/webhooks")
      return res.data ?? []
    },
  })

  const [form, setForm] = useState<Omit<WebhookSub, "id"> & { id: string }>({
    ...empty,
    id: "",
  })
  const [open, setOpen] = useState(false)

  const saveMut = useMutation({
    mutationFn: async (s: Omit<WebhookSub, "id"> & { id: string }) => {
      if (s.id) {
        return api.put(`/admin/api/webhooks/${s.id}`, s)
      }
      const { id: _id, ...rest } = s
      void _id
      return api.post("/admin/api/webhooks", rest)
    },
    onSuccess: () => {
      toast.success("已保存")
      setOpen(false)
      qc.invalidateQueries({ queryKey: ["webhooks"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const delMut = useMutation({
    mutationFn: (id: string) => api.del(`/admin/api/webhooks/${id}`),
    onSuccess: () => {
      toast.success("已删除")
      qc.invalidateQueries({ queryKey: ["webhooks"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const subs = data ?? []

  const toggleEvent = (event: string) => {
    const current = form.events
      .split(",")
      .map((e) => e.trim())
      .filter(Boolean)
    const next = current.includes(event)
      ? current.filter((e) => e !== event)
      : [...current, event]
    setForm({ ...form, events: next.join(",") })
  }

  const selectedEvents = form.events
    .split(",")
    .map((e) => e.trim())
    .filter(Boolean)

  return (
    <div className="space-y-4 p-4 md:p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold">
            <Webhook className="h-6 w-6" />
            Webhook 订阅
          </h1>
          <p className="text-sm text-muted-foreground">
            订阅平台事件，事件触发时以 JSON POST 推送到目标 URL，
            携带 <code className="font-mono">X-Signature-256</code> HMAC-SHA256 签名头
          </p>
        </div>
        <Button
          size="sm"
          onClick={() => {
            setForm({ ...empty, id: "" })
            setOpen(true)
          }}
        >
          <Plus /> 新增
        </Button>
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>名称</TableHead>
              <TableHead>目标 URL</TableHead>
              <TableHead>订阅事件</TableHead>
              <TableHead>租户（空=全局）</TableHead>
              <TableHead>状态</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={6} className="py-8 text-center text-muted-foreground">
                  加载中…
                </TableCell>
              </TableRow>
            ) : subs.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="py-8 text-center text-muted-foreground">
                  暂无 Webhook 订阅
                </TableCell>
              </TableRow>
            ) : (
              subs.map((s) => {
                const events = s.events
                  .split(",")
                  .map((e) => e.trim())
                  .filter(Boolean)
                return (
                  <TableRow key={s.id}>
                    <TableCell className="font-medium">{s.name}</TableCell>
                    <TableCell className="max-w-[280px] truncate font-mono text-xs text-muted-foreground">
                      {s.url}
                    </TableCell>
                    <TableCell>
                      <div className="flex max-w-[240px] flex-wrap gap-1">
                        {events.length === 0 ? (
                          <span className="text-xs text-muted-foreground">—</span>
                        ) : (
                          events.map((e) => (
                            <Badge key={e} variant="secondary" className="font-mono text-xs">
                              {e}
                            </Badge>
                          ))
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {s.tenantID || "全局"}
                    </TableCell>
                    <TableCell>
                      <Badge variant={s.enable ? "default" : "outline"}>
                        {s.enable ? "启用" : "停用"}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        size="icon"
                        variant="ghost"
                        onClick={() => {
                          setForm({ ...s })
                          setOpen(true)
                        }}
                      >
                        <Pencil />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        onClick={() => {
                          if (confirm("确认删除此 Webhook 订阅？")) delMut.mutate(s.id)
                        }}
                      >
                        <Trash2 className="text-destructive" />
                      </Button>
                    </TableCell>
                  </TableRow>
                )
              })
            )}
          </TableBody>
        </Table>
      </div>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{form.id ? "编辑 Webhook" : "新增 Webhook"}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1">
              <Label className="text-xs">名称</Label>
              <Input
                value={form.name}
                placeholder="Slack 告警 / 钉钉机器人"
                onChange={(e) => setForm({ ...form, name: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">目标 URL</Label>
              <Input
                value={form.url}
                placeholder="https://hooks.example.com/uc"
                onChange={(e) => setForm({ ...form, url: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">订阅事件</Label>
              <div className="flex flex-wrap gap-2">
                {KNOWN_EVENTS.map((event) => {
                  const active = selectedEvents.includes(event)
                  return (
                    <Badge
                      key={event}
                      variant={active ? "default" : "outline"}
                      className="cursor-pointer font-mono text-xs select-none"
                      onClick={() => toggleEvent(event)}
                    >
                      {event}
                    </Badge>
                  )
                })}
              </div>
              <Textarea
                className="mt-1 font-mono text-xs"
                rows={2}
                value={form.events}
                placeholder="user.created,role.updated（逗号分隔，也可自定义事件名）"
                onChange={(e) => setForm({ ...form, events: e.target.value })}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label className="text-xs">租户 ID（空=全局）</Label>
                <Input
                  value={form.tenantID}
                  onChange={(e) => setForm({ ...form, tenantID: e.target.value })}
                />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">签名密钥 Secret</Label>
                <Input
                  value={form.secret}
                  placeholder="留空=不签名"
                  onChange={(e) => setForm({ ...form, secret: e.target.value })}
                />
              </div>
            </div>
            <div className="flex items-center justify-between rounded-md border px-3 py-2">
              <Label className="text-xs">启用此订阅</Label>
              <Switch
                checked={form.enable}
                onCheckedChange={(v) => setForm({ ...form, enable: v })}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              取消
            </Button>
            <Button
              onClick={() => {
                if (!form.name.trim() || !form.url.trim()) {
                  toast.error("名称和目标 URL 不能为空")
                  return
                }
                saveMut.mutate(form)
              }}
              disabled={saveMut.isPending}
            >
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
