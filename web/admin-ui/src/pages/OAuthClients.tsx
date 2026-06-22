import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Copy, Pencil, Plus, RefreshCw, Trash2 } from "lucide-react"

import { api } from "@/lib/api"
import type { OAuthClient } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
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
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

interface ClientFormState {
  id?: string
  name: string
  redirectURIs: string // newline-separated
  grantTypes: string
  scopes: string
  enable: boolean
}

const emptyForm: ClientFormState = {
  name: "",
  redirectURIs: "",
  grantTypes: "authorization_code\nrefresh_token",
  scopes: "openid profile email",
  enable: true,
}

function splitLines(v: string): string[] {
  return v
    .split(/[\n,]/)
    .map((s) => s.trim())
    .filter(Boolean)
}

function joinLines(arr: string[]): string {
  return Array.isArray(arr) ? arr.join("\n") : String(arr ?? "")
}

export default function OAuthClients() {
  const qc = useQueryClient()
  const issuer = window.location.origin

  const [editOpen, setEditOpen] = useState(false)
  const [form, setForm] = useState<ClientFormState>(emptyForm)
  const [secretShown, setSecretShown] = useState<{ id: string; secret: string } | null>(null)

  const { data: clients, isLoading } = useQuery<OAuthClient[]>({
    queryKey: ["oauth-clients"],
    queryFn: () => api.get<OAuthClient[]>("/admin/api/oauth-clients"),
  })

  const addMutation = useMutation({
    mutationFn: (body: {
      name: string
      redirectURIs: string[]
      grantTypes: string[]
      scopes: string[]
    }) => api.post<{ id: string; secret: string }>("/admin/api/oauth-clients", body),
    onSuccess: (d) => {
      toast.success("客户端已创建")
      setEditOpen(false)
      setSecretShown(d)
      qc.invalidateQueries({ queryKey: ["oauth-clients"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const updateMutation = useMutation({
    mutationFn: (body: { id: string; name: string; redirectURIs: string[]; grantTypes: string[]; scopes: string[]; enable: boolean }) =>
      api.put(`/admin/api/oauth-clients/${body.id}`, body),
    onSuccess: () => {
      toast.success("已更新")
      setEditOpen(false)
      qc.invalidateQueries({ queryKey: ["oauth-clients"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const rotateMutation = useMutation({
    mutationFn: (id: string) =>
      api.post<{ secret: string }>(`/admin/api/oauth-clients/${id}/rotate-secret`),
    onSuccess: (d, id) => {
      toast.success("密钥已轮换")
      setSecretShown({ id, secret: d.secret })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.del(`/admin/api/oauth-clients/${id}`),
    onSuccess: () => {
      toast.success("已删除")
      qc.invalidateQueries({ queryKey: ["oauth-clients"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  function openAdd() {
    setForm({ ...emptyForm })
    setEditOpen(true)
  }

  function openEdit(c: OAuthClient) {
    setForm({
      id: c.id,
      name: c.name,
      redirectURIs: joinLines(
        typeof c.redirectURIs === "string" ? c.redirectURIs.split("\n") : c.redirectURIs,
      ),
      grantTypes:
        typeof c.grantTypes === "string" && c.grantTypes.includes("\n")
          ? c.grantTypes
          : joinLines(splitLines(c.grantTypes)),
      scopes:
        typeof c.scopes === "string" && c.scopes.includes("\n")
          ? c.scopes
          : joinLines(splitLines(c.scopes)),
      enable: c.enable,
    })
    setEditOpen(true)
  }

  function submit() {
    if (!form.name.trim()) {
      toast.error("请填写名称")
      return
    }
    const payload = {
      name: form.name.trim(),
      redirectURIs: splitLines(form.redirectURIs),
      grantTypes: splitLines(form.grantTypes),
      scopes: splitLines(form.scopes),
      enable: form.enable,
    }
    if (form.id) updateMutation.mutate({ ...payload, id: form.id })
    else addMutation.mutate(payload)
  }

  const endpoints = [
    { label: "Issuer", url: issuer },
    { label: "Discovery", url: `${issuer}/.well-known/openid-configuration` },
    { label: "JWKS", url: `${issuer}/.well-known/jwks.json` },
    { label: "Authorize", url: `${issuer}/oauth/authorize` },
    { label: "Token", url: `${issuer}/oauth/token` },
    { label: "UserInfo", url: `${issuer}/oauth/userinfo` },
  ]

  return (
    <div className="space-y-6 p-4 md:p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-bold">OAuth 客户端</h1>
        <Button size="sm" onClick={openAdd}>
          <Plus /> 新增客户端
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>OIDC 端点</CardTitle>
          <CardDescription>基于当前 origin 推断（{issuer}）</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-2 sm:grid-cols-2">
          {endpoints.map((e) => (
            <div key={e.label} className="flex items-center justify-between gap-2 rounded-md border px-3 py-2">
              <div className="min-w-0">
                <div className="text-xs text-muted-foreground">{e.label}</div>
                <div className="truncate font-mono text-xs">{e.url}</div>
              </div>
              <Button
                size="icon"
                variant="ghost"
                title="复制"
                onClick={() => {
                  navigator.clipboard?.writeText(e.url)
                  toast.success("已复制")
                }}
              >
                <Copy />
              </Button>
            </div>
          ))}
        </CardContent>
      </Card>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>ID</TableHead>
              <TableHead>名称</TableHead>
              <TableHead>Redirect URIs</TableHead>
              <TableHead>Grant Types</TableHead>
              <TableHead>Scopes</TableHead>
              <TableHead>状态</TableHead>
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
            ) : (clients ?? []).length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className="py-8 text-center text-muted-foreground">
                  暂无数据
                </TableCell>
              </TableRow>
            ) : (
              (clients ?? []).map((c) => (
                <TableRow key={c.id}>
                  <TableCell className="font-mono text-xs">{c.id}</TableCell>
                  <TableCell className="font-medium">{c.name}</TableCell>
                  <TableCell className="max-w-[200px] truncate text-xs" title={String(c.redirectURIs)}>
                    {String(c.redirectURIs)}
                  </TableCell>
                  <TableCell className="max-w-[160px] truncate text-xs" title={String(c.grantTypes)}>
                    {String(c.grantTypes)}
                  </TableCell>
                  <TableCell className="max-w-[140px] truncate text-xs" title={String(c.scopes)}>
                    {String(c.scopes)}
                  </TableCell>
                  <TableCell>
                    {c.enable ? <Badge>启用</Badge> : <Badge variant="secondary">禁用</Badge>}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="inline-flex gap-1">
                      <Button size="icon" variant="ghost" title="编辑" onClick={() => openEdit(c)}>
                        <Pencil />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        title="轮换密钥"
                        onClick={() => {
                          if (window.confirm(`确认轮换 ${c.name} 的密钥？旧密钥将立即失效。`))
                            rotateMutation.mutate(c.id)
                        }}
                      >
                        <RefreshCw />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        title="删除"
                        onClick={() => {
                          if (window.confirm(`确认删除客户端 ${c.name}？`))
                            deleteMutation.mutate(c.id)
                        }}
                      >
                        <Trash2 className="text-destructive" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {/* Add / Edit */}
      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>{form.id ? "编辑客户端" : "新增客户端"}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1">
              <Label>名称</Label>
              <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
            </div>
            <div className="space-y-1">
              <Label>Redirect URIs（每行一个）</Label>
              <Textarea
                rows={3}
                className="font-mono text-xs"
                value={form.redirectURIs}
                onChange={(e) => setForm({ ...form, redirectURIs: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>Grant Types（每行一个）</Label>
              <Textarea
                rows={3}
                className="font-mono text-xs"
                value={form.grantTypes}
                onChange={(e) => setForm({ ...form, grantTypes: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>Scopes（空格或换行分隔）</Label>
              <Textarea
                rows={2}
                className="font-mono text-xs"
                value={form.scopes}
                onChange={(e) => setForm({ ...form, scopes: e.target.value })}
              />
            </div>
            {form.id && (
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={form.enable}
                  onChange={(e) => setForm({ ...form, enable: e.target.checked })}
                />
                启用
              </label>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditOpen(false)}>
              取消
            </Button>
            <Button onClick={submit} disabled={addMutation.isPending || updateMutation.isPending}>
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Secret once-off display */}
      <Dialog open={!!secretShown} onOpenChange={(o) => !o && setSecretShown(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>客户端密钥（仅显示一次）</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="rounded-md border border-destructive/50 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              请立即保存此密钥，关闭后将无法再次查看。
            </div>
            <div className="space-y-1">
              <Label className="text-xs">Client ID</Label>
              <Input className="font-mono text-xs" readOnly value={secretShown?.id ?? ""} />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">Client Secret</Label>
              <div className="flex gap-2">
                <Input className="font-mono text-xs" readOnly value={secretShown?.secret ?? ""} />
                <Button
                  variant="outline"
                  size="icon"
                  title="复制"
                  onClick={() => {
                    if (secretShown) {
                      navigator.clipboard?.writeText(secretShown.secret)
                      toast.success("已复制")
                    }
                  }}
                >
                  <Copy />
                </Button>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button onClick={() => setSecretShown(null)}>我已保存</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
