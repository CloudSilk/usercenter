import { useRef, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import {
  Download,
  KeyRound,
  Pencil,
  Plus,
  Power,
  Trash2,
  Upload,
} from "lucide-react"

import { api, getToken } from "@/lib/api"
import type { Role, User } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { Switch } from "@/components/ui/switch"
import { Checkbox } from "@/components/ui/checkbox"
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

interface UserListResponse {
  data: User[]
  records: number
}

const emptyForm: UserFormState = {
  id: "",
  userName: "",
  nickname: "",
  mobile: "",
  email: "",
  title: "",
  realName: "",
  eid: "",
  enable: true,
  roleIDs: [],
}

interface UserFormState {
  id: string
  userName: string
  nickname: string
  mobile: string
  email: string
  title: string
  realName: string
  eid: string
  enable: boolean
  roleIDs: string[]
}

function fmtTime(v?: string): string {
  if (!v) return "-"
  const n = Number(v)
  const d = !Number.isNaN(n) && String(v).length >= 10 ? new Date(n * 1000) : new Date(v)
  const t = d.getTime()
  return Number.isNaN(t) ? String(v) : d.toLocaleString()
}

export default function Users() {
  const qc = useQueryClient()
  const fileInputRef = useRef<HTMLInputElement | null>(null)

  const [search, setSearch] = useState({ userName: "", nickname: "", mobile: "" })
  const [applied, setApplied] = useState({ userName: "", nickname: "", mobile: "" })
  const [pageIndex, setPageIndex] = useState(1)
  const pageSize = 10

  const [selected, setSelected] = useState<Record<string, boolean>>({})
  const [editOpen, setEditOpen] = useState(false)
  const [form, setForm] = useState<UserFormState>(emptyForm)
  const [resetTarget, setResetTarget] = useState<User | null>(null)
  const [resetPwd, setResetPwd] = useState("")

  const listKey = ["users", applied, pageIndex, pageSize]

  const { data, isLoading } = useQuery<UserListResponse>({
    queryKey: listKey,
    queryFn: () =>
      api.get<UserListResponse>("/api/core/auth/user/query", {
        pageIndex,
        pageSize,
        userName: applied.userName || undefined,
        nickname: applied.nickname || undefined,
        mobile: applied.mobile || undefined,
      }),
  })

  const { data: roles } = useQuery<Role[]>({
    queryKey: ["roles-for-user-form"],
    queryFn: () => api.get<Role[]>("/api/core/auth/user/role/query"),
  })

  const users = data?.data ?? []
  const total = data?.records ?? 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const selectedIds = users.filter((u) => selected[u.id]).map((u) => u.id)

  function toggleRole(id: string, on: boolean) {
    setForm((f) => ({
      ...f,
      roleIDs: on
        ? [...new Set([...f.roleIDs, id])]
        : f.roleIDs.filter((r) => r !== id),
    }))
  }

  const addMutation = useMutation({
    mutationFn: (body: Omit<UserFormState, "id">) =>
      api.post("/api/core/auth/user/add", body),
    onSuccess: () => {
      toast.success("用户已创建")
      setEditOpen(false)
      qc.invalidateQueries({ queryKey: ["users"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const updateMutation = useMutation({
    mutationFn: (body: UserFormState) => api.put("/api/core/auth/user/update", body),
    onSuccess: () => {
      toast.success("用户已更新")
      setEditOpen(false)
      qc.invalidateQueries({ queryKey: ["users"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.del("/api/core/auth/user/delete", { id }),
    onSuccess: () => {
      toast.success("用户已删除")
      qc.invalidateQueries({ queryKey: ["users"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const enableMutation = useMutation({
    mutationFn: (vars: { id: string; enable: boolean }) =>
      api.post("/api/core/auth/user/enable", vars),
    onSuccess: (_d, vars) => {
      toast.success(vars.enable ? "已启用" : "已禁用")
      qc.invalidateQueries({ queryKey: ["users"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const resetMutation = useMutation({
    mutationFn: (vars: { id: string; password: string }) =>
      api.post("/api/core/auth/user/resetpwd", vars),
    onSuccess: () => {
      toast.success("密码已重置")
      setResetTarget(null)
      setResetPwd("")
    },
    onError: (e: Error) => toast.error(e.message),
  })

  function openAdd() {
    setForm({ ...emptyForm })
    setEditOpen(true)
  }

  function openEdit(u: User) {
    setForm({
      id: u.id,
      userName: u.userName ?? "",
      nickname: u.nickname ?? "",
      mobile: u.mobile ?? "",
      email: u.email ?? "",
      title: u.title ?? "",
      realName: u.realName ?? "",
      eid: u.eid ?? "",
      enable: !!u.enable,
      roleIDs: u.roleIDs ?? [],
    })
    setEditOpen(true)
  }

  function submitForm() {
    if (!form.userName.trim()) {
      toast.error("请填写用户名")
      return
    }
    if (form.id) updateMutation.mutate(form)
    else {
      const { id: _id, ...body } = form
      void _id
      addMutation.mutate(body)
    }
  }

  function bulkEnable(enable: boolean) {
    if (!selectedIds.length) return
    Promise.all(selectedIds.map((id) => enableMutation.mutateAsync({ id, enable })))
      .then(() => {
        toast.success(enable ? "批量启用完成" : "批量禁用完成")
        setSelected({})
      })
      .catch((e: Error) => toast.error(e.message))
  }

  function bulkDelete() {
    if (!selectedIds.length) return
    if (!window.confirm(`确认删除选中的 ${selectedIds.length} 个用户？`)) return
    Promise.all(selectedIds.map((id) => deleteMutation.mutateAsync(id)))
      .then(() => {
        toast.success("批量删除完成")
        setSelected({})
      })
      .catch((e: Error) => toast.error(e.message))
  }

  async function handleExport() {
    try {
      const token = getToken()
      const res = await fetch("/api/core/auth/user/export", {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      })
      if (!res.ok) throw new Error("导出失败")
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement("a")
      a.href = url
      a.download = "users.json"
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
      toast.success("已导出")
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "导出失败")
    }
  }

  async function handleImportFile(file: File) {
    try {
      const fd = new FormData()
      fd.append("files", file)
      const token = getToken()
      const res = await fetch("/api/core/auth/user/import", {
        method: "POST",
        headers: token ? { Authorization: `Bearer ${token}` } : {},
        body: fd,
      })
      const json = await res.json()
      if (json.code !== 0) throw new Error(json.message || "导入失败")
      toast.success(json.message || "导入成功")
      qc.invalidateQueries({ queryKey: ["users"] })
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "导入失败")
    }
  }

  const allChecked = users.length > 0 && users.every((u) => selected[u.id])
  const someChecked = users.some((u) => selected[u.id]) && !allChecked

  function toggleAll(checked: boolean) {
    setSelected((s) => {
      const next = { ...s }
      for (const u of users) next[u.id] = checked
      return next
    })
  }

  return (
    <div className="space-y-4 p-4 md:p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-bold">用户管理</h1>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" onClick={handleExport}>
            <Download /> 导出
          </Button>
          <Button variant="outline" size="sm" onClick={() => fileInputRef.current?.click()}>
            <Upload /> 导入
          </Button>
          <input
            ref={fileInputRef}
            type="file"
            accept=".json,.csv"
            className="hidden"
            onChange={(e) => {
              const f = e.target.files?.[0]
              if (f) handleImportFile(f)
              e.target.value = ""
            }}
          />
          <Button size="sm" onClick={openAdd}>
            <Plus /> 新增用户
          </Button>
        </div>
      </div>

      <div className="flex flex-wrap items-end gap-2">
        <div className="space-y-1">
          <Label className="text-xs">用户名</Label>
          <Input
            className="h-8 w-44"
            value={search.userName}
            onChange={(e) => setSearch({ ...search, userName: e.target.value })}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                setApplied({ ...search })
                setPageIndex(1)
              }
            }}
          />
        </div>
        <div className="space-y-1">
          <Label className="text-xs">昵称</Label>
          <Input
            className="h-8 w-44"
            value={search.nickname}
            onChange={(e) => setSearch({ ...search, nickname: e.target.value })}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                setApplied({ ...search })
                setPageIndex(1)
              }
            }}
          />
        </div>
        <div className="space-y-1">
          <Label className="text-xs">手机号</Label>
          <Input
            className="h-8 w-44"
            value={search.mobile}
            onChange={(e) => setSearch({ ...search, mobile: e.target.value })}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                setApplied({ ...search })
                setPageIndex(1)
              }
            }}
          />
        </div>
        <Button
          size="sm"
          onClick={() => {
            setApplied({ ...search })
            setPageIndex(1)
          }}
        >
          查询
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={() => {
            const empty = { userName: "", nickname: "", mobile: "" }
            setSearch(empty)
            setApplied(empty)
            setPageIndex(1)
          }}
        >
          重置
        </Button>

        {selectedIds.length > 0 && (
          <div className="flex items-center gap-2 border-l pl-3">
            <span className="text-sm text-muted-foreground">
              已选 {selectedIds.length}
            </span>
            <Button size="sm" variant="outline" onClick={() => bulkEnable(true)}>
              <Power /> 启用
            </Button>
            <Button size="sm" variant="outline" onClick={() => bulkEnable(false)}>
              <Power /> 禁用
            </Button>
            <Button size="sm" variant="destructive" onClick={bulkDelete}>
              <Trash2 /> 删除
            </Button>
          </div>
        )}
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">
                <Checkbox
                  checked={allChecked ? true : someChecked ? "indeterminate" : false}
                  onCheckedChange={(v) => toggleAll(v === true)}
                />
              </TableHead>
              <TableHead>用户名</TableHead>
              <TableHead>昵称</TableHead>
              <TableHead>手机号</TableHead>
              <TableHead>邮箱</TableHead>
              <TableHead>职位</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>创建时间</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={9} className="py-8 text-center text-muted-foreground">
                  加载中…
                </TableCell>
              </TableRow>
            ) : users.length === 0 ? (
              <TableRow>
                <TableCell colSpan={9} className="py-8 text-center text-muted-foreground">
                  暂无数据
                </TableCell>
              </TableRow>
            ) : (
              users.map((u) => (
                <TableRow key={u.id} data-state={selected[u.id] ? "selected" : undefined}>
                  <TableCell>
                    <Checkbox
                      checked={!!selected[u.id]}
                      onCheckedChange={(v) =>
                        setSelected((s) => ({ ...s, [u.id]: v === true }))
                      }
                    />
                  </TableCell>
                  <TableCell className="font-medium">{u.userName}</TableCell>
                  <TableCell>{u.nickname || "-"}</TableCell>
                  <TableCell>{u.mobile || "-"}</TableCell>
                  <TableCell>{u.email || "-"}</TableCell>
                  <TableCell>{u.title || "-"}</TableCell>
                  <TableCell>
                    {u.enable ? (
                      <Badge>启用</Badge>
                    ) : (
                      <Badge variant="secondary">禁用</Badge>
                    )}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {fmtTime(u.createdAt)}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="inline-flex gap-1">
                      <Button
                        size="icon"
                        variant="ghost"
                        title="编辑"
                        onClick={() => openEdit(u)}
                      >
                        <Pencil />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        title={u.enable ? "禁用" : "启用"}
                        onClick={() =>
                          enableMutation.mutate({ id: u.id, enable: !u.enable })
                        }
                      >
                        <Power />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        title="重置密码"
                        onClick={() => {
                          setResetTarget(u)
                          setResetPwd("")
                        }}
                      >
                        <KeyRound />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        title="删除"
                        onClick={() => {
                          if (window.confirm(`确认删除用户 ${u.userName}？`))
                            deleteMutation.mutate(u.id)
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

      {/* Add / Edit dialog */}
      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>{form.id ? "编辑用户" : "新增用户"}</DialogTitle>
          </DialogHeader>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1">
              <Label>用户名</Label>
              <Input
                value={form.userName}
                disabled={!!form.id}
                onChange={(e) => setForm({ ...form, userName: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>昵称</Label>
              <Input
                value={form.nickname}
                onChange={(e) => setForm({ ...form, nickname: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>手机号</Label>
              <Input
                value={form.mobile}
                onChange={(e) => setForm({ ...form, mobile: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>邮箱</Label>
              <Input
                value={form.email}
                onChange={(e) => setForm({ ...form, email: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>职位</Label>
              <Input
                value={form.title}
                onChange={(e) => setForm({ ...form, title: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>真实姓名</Label>
              <Input
                value={form.realName}
                onChange={(e) => setForm({ ...form, realName: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>工号</Label>
              <Input
                value={form.eid}
                onChange={(e) => setForm({ ...form, eid: e.target.value })}
              />
            </div>
            <div className="flex items-center gap-2 pt-6">
              <Switch
                checked={form.enable}
                onCheckedChange={(v) => setForm({ ...form, enable: v })}
              />
              <Label>启用</Label>
            </div>
          </div>

          <div className="space-y-2">
            <Label>角色</Label>
            <div className="max-h-36 space-y-1 overflow-y-auto rounded-md border p-2">
              {(roles ?? []).length === 0 ? (
                <div className="py-2 text-center text-sm text-muted-foreground">
                  暂无可选角色
                </div>
              ) : (
                (roles ?? []).map((r) => (
                  <label
                    key={r.id}
                    className="flex cursor-pointer items-center gap-2 rounded px-1 py-1 hover:bg-accent"
                  >
                    <Checkbox
                      checked={form.roleIDs.includes(r.id)}
                      onCheckedChange={(v) => toggleRole(r.id, v === true)}
                    />
                    <span className="text-sm">{r.name}</span>
                  </label>
                ))
              )}
            </div>
            {form.roleIDs.length > 0 && (
              <div className="flex flex-wrap gap-1">
                {(roles ?? [])
                  .filter((r) => form.roleIDs.includes(r.id))
                  .map((r) => (
                    <Badge key={r.id} variant="secondary">
                      {r.name}
                      <button
                        type="button"
                        className="ml-1 hover:text-destructive"
                        onClick={() => toggleRole(r.id, false)}
                      >
                        ×
                      </button>
                    </Badge>
                  ))}
              </div>
            )}
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setEditOpen(false)}>
              取消
            </Button>
            <Button
              onClick={submitForm}
              disabled={addMutation.isPending || updateMutation.isPending}
            >
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Reset password dialog */}
      <Dialog
        open={!!resetTarget}
        onOpenChange={(o) => {
          if (!o) {
            setResetTarget(null)
            setResetPwd("")
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>重置密码 - {resetTarget?.userName}</DialogTitle>
          </DialogHeader>
          <div className="space-y-1">
            <Label>新密码（留空则随机生成）</Label>
            <Input
              value={resetPwd}
              placeholder="留空随机生成"
              onChange={(e) => setResetPwd(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setResetTarget(null)
                setResetPwd("")
              }}
            >
              取消
            </Button>
            <Button
              onClick={() => {
                if (resetTarget)
                  resetMutation.mutate({ id: resetTarget.id, password: resetPwd })
              }}
              disabled={resetMutation.isPending}
            >
              确认重置
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
