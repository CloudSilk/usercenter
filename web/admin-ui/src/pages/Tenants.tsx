import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Pencil, Plus, Power, Trash2 } from "lucide-react"

import { api } from "@/lib/api"
import type { Tenant } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { Switch } from "@/components/ui/switch"
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

interface TenantListResponse {
  data: Tenant[]
  records: number
}

interface TenantFormState {
  id: string
  name: string
  contact: string
  cellPhone: string
  address: string
  staffSize: number
  enable: boolean
}

const emptyForm: TenantFormState = {
  id: "",
  name: "",
  contact: "",
  cellPhone: "",
  address: "",
  staffSize: 0,
  enable: true,
}

export default function Tenants() {
  const qc = useQueryClient()

  const [search, setSearch] = useState("")
  const [applied, setApplied] = useState("")
  const [pageIndex, setPageIndex] = useState(1)
  const pageSize = 10

  const [editOpen, setEditOpen] = useState(false)
  const [form, setForm] = useState<TenantFormState>(emptyForm)

  const listKey = ["tenants", applied, pageIndex, pageSize]

  const { data, isLoading } = useQuery<TenantListResponse>({
    queryKey: listKey,
    queryFn: () =>
      api.get<TenantListResponse>("/api/core/auth/tenant/query", {
        pageIndex,
        pageSize,
        name: applied || undefined,
      }),
  })

  const tenants = data?.data ?? []
  const total = data?.records ?? 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  const addMutation = useMutation({
    mutationFn: (body: Omit<TenantFormState, "id">) =>
      api.post("/api/core/auth/tenant/add", body),
    onSuccess: () => {
      toast.success("租户已创建")
      setEditOpen(false)
      qc.invalidateQueries({ queryKey: ["tenants"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const updateMutation = useMutation({
    mutationFn: (body: TenantFormState) =>
      api.put("/api/core/auth/tenant/update", body),
    onSuccess: () => {
      toast.success("租户已更新")
      setEditOpen(false)
      qc.invalidateQueries({ queryKey: ["tenants"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.del("/api/core/auth/tenant/delete", { id }),
    onSuccess: () => {
      toast.success("租户已删除")
      qc.invalidateQueries({ queryKey: ["tenants"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const enableMutation = useMutation({
    mutationFn: (vars: { id: string; enable: boolean }) =>
      api.post("/api/core/auth/tenant/enable", vars),
    onSuccess: (_d, vars) => {
      toast.success(vars.enable ? "已启用" : "已禁用")
      qc.invalidateQueries({ queryKey: ["tenants"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  function openAdd() {
    setForm({ ...emptyForm })
    setEditOpen(true)
  }

  function openEdit(t: Tenant) {
    setForm({
      id: t.id,
      name: t.name ?? "",
      contact: t.contact ?? "",
      cellPhone: t.cellPhone ?? "",
      address: "",
      staffSize: t.staffSize ?? 0,
      enable: !!t.enable,
    })
    setEditOpen(true)
  }

  function submitForm() {
    if (!form.name.trim()) {
      toast.error("请填写租户名称")
      return
    }
    if (form.id) updateMutation.mutate(form)
    else {
      const { id: _id, ...body } = form
      void _id
      addMutation.mutate(body)
    }
  }

  return (
    <div className="space-y-4 p-4 md:p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-bold">租户管理</h1>
        <Button size="sm" onClick={openAdd}>
          <Plus /> 新增租户
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
              <TableHead>联系人</TableHead>
              <TableHead>电话</TableHead>
              <TableHead>规模</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>用户数</TableHead>
              <TableHead>角色数</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={8} className="py-8 text-center text-muted-foreground">
                  加载中…
                </TableCell>
              </TableRow>
            ) : tenants.length === 0 ? (
              <TableRow>
                <TableCell colSpan={8} className="py-8 text-center text-muted-foreground">
                  暂无数据
                </TableCell>
              </TableRow>
            ) : (
              tenants.map((t) => (
                <TableRow key={t.id}>
                  <TableCell className="font-medium">{t.name}</TableCell>
                  <TableCell>{t.contact || "-"}</TableCell>
                  <TableCell>{t.cellPhone || "-"}</TableCell>
                  <TableCell>{t.staffSize ?? 0}</TableCell>
                  <TableCell>
                    {t.enable ? (
                      <Badge>启用</Badge>
                    ) : (
                      <Badge variant="secondary">禁用</Badge>
                    )}
                  </TableCell>
                  <TableCell>{t.userCount ?? 0}</TableCell>
                  <TableCell>{t.roleCount ?? 0}</TableCell>
                  <TableCell className="text-right">
                    <div className="inline-flex gap-1">
                      <Button size="icon" variant="ghost" title="编辑" onClick={() => openEdit(t)}>
                        <Pencil />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        title={t.enable ? "禁用" : "启用"}
                        onClick={() =>
                          enableMutation.mutate({ id: t.id, enable: !t.enable })
                        }
                      >
                        <Power />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        title="删除"
                        onClick={() => {
                          if (window.confirm(`确认删除租户 ${t.name}？`))
                            deleteMutation.mutate(t.id)
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

      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>{form.id ? "编辑租户" : "新增租户"}</DialogTitle>
          </DialogHeader>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1">
              <Label>名称</Label>
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>联系人</Label>
              <Input
                value={form.contact}
                onChange={(e) => setForm({ ...form, contact: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>电话</Label>
              <Input
                value={form.cellPhone}
                onChange={(e) => setForm({ ...form, cellPhone: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>规模</Label>
              <Input
                type="number"
                value={form.staffSize}
                onChange={(e) =>
                  setForm({ ...form, staffSize: Number(e.target.value) || 0 })
                }
              />
            </div>
            <div className="col-span-2 space-y-1">
              <Label>地址</Label>
              <Input
                value={form.address}
                onChange={(e) => setForm({ ...form, address: e.target.value })}
              />
            </div>
            <div className="col-span-2 flex items-center gap-2">
              <Switch
                checked={form.enable}
                onCheckedChange={(v) => setForm({ ...form, enable: v })}
              />
              <Label>启用</Label>
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
    </div>
  )
}
