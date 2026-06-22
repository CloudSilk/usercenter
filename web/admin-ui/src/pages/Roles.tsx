import { useMemo, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Pencil, Plus, ShieldCheck, Trash2 } from "lucide-react"

import { api } from "@/lib/api"
import type { MenuInfo, Role, RoleMenu } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
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

interface RoleListResponse {
  data: Role[]
  records: number
}

interface RoleInfo {
  id: string
  name: string
  code: string
  description: string
  roleMenus: RoleMenu[]
}

interface RoleFormState {
  id: string
  name: string
  code: string
  description: string
}

const emptyForm: RoleFormState = { id: "", name: "", code: "", description: "" }

/** Flatten a menu tree into a list with depth for indented rendering. */
interface FlatMenu {
  id: string
  name: string
  depth: number
}

function flattenMenu(nodes: MenuInfo[], depth = 0, acc: FlatMenu[] = []): FlatMenu[] {
  for (const n of nodes ?? []) {
    acc.push({ id: n.id, name: n.name, depth })
    if (n.children && n.children.length) {
      flattenMenu(n.children, depth + 1, acc)
    }
  }
  return acc
}

export default function Roles() {
  const qc = useQueryClient()

  const [search, setSearch] = useState("")
  const [applied, setApplied] = useState("")
  const [pageIndex, setPageIndex] = useState(1)
  const pageSize = 10

  const [editOpen, setEditOpen] = useState(false)
  const [form, setForm] = useState<RoleFormState>(emptyForm)

  // Menu authorization dialog state
  const [authTarget, setAuthTarget] = useState<Role | null>(null)
  const [checkedMenus, setCheckedMenus] = useState<Record<string, boolean>>({})

  const listKey = ["roles", applied, pageIndex, pageSize]

  const { data, isLoading } = useQuery<RoleListResponse>({
    queryKey: listKey,
    queryFn: () =>
      api.get<RoleListResponse>("/api/core/auth/role/query", {
        pageIndex,
        pageSize,
        name: applied || undefined,
      }),
  })

  const roles = data?.data ?? []
  const total = data?.records ?? 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  const addMutation = useMutation({
    mutationFn: (body: Omit<RoleFormState, "id">) =>
      api.post("/api/core/auth/role/add", body),
    onSuccess: () => {
      toast.success("角色已创建")
      setEditOpen(false)
      qc.invalidateQueries({ queryKey: ["roles"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const updateMutation = useMutation({
    mutationFn: (body: RoleFormState) =>
      api.put("/api/core/auth/role/update", body),
    onSuccess: () => {
      toast.success("角色已更新")
      setEditOpen(false)
      qc.invalidateQueries({ queryKey: ["roles"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.del("/api/core/auth/role/delete", { id }),
    onSuccess: () => {
      toast.success("角色已删除")
      qc.invalidateQueries({ queryKey: ["roles"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  // Menu tree (always loaded; cheap)
  const { data: menuTree } = useQuery<MenuInfo[]>({
    queryKey: ["menu-tree"],
    queryFn: () => api.get<MenuInfo[]>("/api/core/auth/menu/tree"),
  })

  const flatMenus = useMemo(() => flattenMenu(menuTree ?? []), [menuTree])

  // Load role detail when opening the auth dialog
  const { data: roleDetail, isFetching: detailLoading } = useQuery<RoleInfo>({
    queryKey: ["role-detail", authTarget?.id],
    queryFn: () =>
      api.get<RoleInfo>("/api/core/auth/role/detail", { id: authTarget!.id }),
    enabled: !!authTarget,
  })

  // Pre-populate checked menus when detail arrives
  const [detailLoadedFor, setDetailLoadedFor] = useState<string | null>(null)
  if (
    roleDetail &&
    authTarget &&
    roleDetail.id === authTarget.id &&
    detailLoadedFor !== authTarget.id
  ) {
    const next: Record<string, boolean> = {}
    for (const rm of roleDetail.roleMenus ?? []) next[rm.menuID] = true
    setCheckedMenus(next)
    setDetailLoadedFor(authTarget.id)
  }

  const authMutation = useMutation({
    mutationFn: (vars: { id: string; name: string; roleMenus: RoleMenu[] }) =>
      api.put("/api/core/auth/role/update", vars),
    onSuccess: () => {
      toast.success("菜单授权已保存")
      setAuthTarget(null)
      setCheckedMenus({})
      setDetailLoadedFor(null)
      qc.invalidateQueries({ queryKey: ["roles"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  function openAdd() {
    setForm({ ...emptyForm })
    setEditOpen(true)
  }

  function openEdit(r: Role) {
    setForm({
      id: r.id,
      name: r.name ?? "",
      code: r.code ?? "",
      description: r.description ?? "",
    })
    setEditOpen(true)
  }

  function submitForm() {
    if (!form.name.trim()) {
      toast.error("请填写角色名称")
      return
    }
    if (form.id) updateMutation.mutate(form)
    else {
      const { id: _id, ...body } = form
      void _id
      addMutation.mutate(body)
    }
  }

  function openAuth(r: Role) {
    setAuthTarget(r)
    setCheckedMenus({})
    setDetailLoadedFor(null)
  }

  function toggleMenu(id: string, on: boolean) {
    // check-strictly: do not cascade to parents/children
    setCheckedMenus((s) => ({ ...s, [id]: on }))
  }

  function selectAllMenus(on: boolean) {
    const next: Record<string, boolean> = {}
    if (on) for (const m of flatMenus) next[m.id] = true
    setCheckedMenus(next)
  }

  function saveAuth() {
    if (!authTarget) return
    const roleMenus: RoleMenu[] = Object.entries(checkedMenus)
      .filter(([, v]) => v)
      .map(([menuID]) => ({ menuID, funcs: "view", show: true }))
    authMutation.mutate({
      id: authTarget.id,
      name: authTarget.name,
      roleMenus,
    })
  }

  const checkedCount = Object.values(checkedMenus).filter(Boolean).length
  const allChecked =
    flatMenus.length > 0 && flatMenus.every((m) => checkedMenus[m.id])
  const someChecked =
    flatMenus.some((m) => checkedMenus[m.id]) && !allChecked

  return (
    <div className="space-y-4 p-4 md:p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-bold">角色管理</h1>
        <Button size="sm" onClick={openAdd}>
          <Plus /> 新增角色
        </Button>
      </div>

      <div className="flex flex-wrap items-end gap-2">
        <div className="space-y-1">
          <Label className="text-xs">名称</Label>
          <Input
            className="h-8 w-56"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                setApplied(search)
                setPageIndex(1)
              }
            }}
          />
        </div>
        <Button
          size="sm"
          onClick={() => {
            setApplied(search)
            setPageIndex(1)
          }}
        >
          查询
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={() => {
            setSearch("")
            setApplied("")
            setPageIndex(1)
          }}
        >
          重置
        </Button>
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>名称</TableHead>
              <TableHead>编码</TableHead>
              <TableHead>描述</TableHead>
              <TableHead>租户</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={5} className="py-8 text-center text-muted-foreground">
                  加载中…
                </TableCell>
              </TableRow>
            ) : roles.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="py-8 text-center text-muted-foreground">
                  暂无数据
                </TableCell>
              </TableRow>
            ) : (
              roles.map((r) => (
                <TableRow key={r.id}>
                  <TableCell className="font-medium">{r.name}</TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">
                    {r.code || "-"}
                  </TableCell>
                  <TableCell className="max-w-xs truncate text-muted-foreground">
                    {r.description || "-"}
                  </TableCell>
                  <TableCell>{r.tenantName || "-"}</TableCell>
                  <TableCell className="text-right">
                    <div className="inline-flex gap-1">
                      <Button size="icon" variant="ghost" title="编辑" onClick={() => openEdit(r)}>
                        <Pencil />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        title="授权菜单"
                        onClick={() => openAuth(r)}
                      >
                        <ShieldCheck />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        title="删除"
                        disabled={r.canDel === false}
                        onClick={() => {
                          if (window.confirm(`确认删除角色 ${r.name}？`))
                            deleteMutation.mutate(r.id)
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
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{form.id ? "编辑角色" : "新增角色"}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-1">
              <Label>名称</Label>
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>编码</Label>
              <Input
                value={form.code}
                onChange={(e) => setForm({ ...form, code: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>描述</Label>
              <Input
                value={form.description}
                onChange={(e) => setForm({ ...form, description: e.target.value })}
              />
            </div>
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

      {/* Menu authorization dialog */}
      <Dialog
        open={!!authTarget}
        onOpenChange={(o) => {
          if (!o) {
            setAuthTarget(null)
            setCheckedMenus({})
            setDetailLoadedFor(null)
          }
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>菜单授权 - {authTarget?.name}</DialogTitle>
          </DialogHeader>

          <div className="flex items-center justify-between text-sm text-muted-foreground">
            <span>
              已选 {checkedCount} / {flatMenus.length}{" "}
              {detailLoading ? "（加载中…）" : ""}
            </span>
            <div className="flex gap-2">
              <Button size="sm" variant="ghost" onClick={() => selectAllMenus(true)}>
                全选
              </Button>
              <Button size="sm" variant="ghost" onClick={() => selectAllMenus(false)}>
                清空
              </Button>
            </div>
          </div>

          <div className="max-h-80 space-y-1 overflow-y-auto rounded-md border p-2">
            {flatMenus.length === 0 ? (
              <div className="py-6 text-center text-sm text-muted-foreground">
                暂无菜单数据
              </div>
            ) : (
              <label className="flex items-center gap-2 border-b pb-2">
                <Checkbox
                  checked={allChecked ? true : someChecked ? "indeterminate" : false}
                  onCheckedChange={(v) => selectAllMenus(v === true)}
                />
                <span className="text-sm font-medium">全选 / 反选</span>
              </label>
            )}
            {flatMenus.map((m) => (
              <label
                key={m.id}
                className="flex cursor-pointer items-center gap-2 rounded px-1 py-1 hover:bg-accent"
                style={{ paddingLeft: `${m.depth * 18 + 4}px` }}
              >
                <Checkbox
                  checked={!!checkedMenus[m.id]}
                  onCheckedChange={(v) => toggleMenu(m.id, v === true)}
                />
                <span className="text-sm">{m.name}</span>
              </label>
            ))}
          </div>

          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setAuthTarget(null)
                setCheckedMenus({})
                setDetailLoadedFor(null)
              }}
            >
              取消
            </Button>
            <Button onClick={saveAuth} disabled={authMutation.isPending}>
              保存授权
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
