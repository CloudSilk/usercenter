import { useMemo, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Pencil, Plus, Search, Trash2 } from "lucide-react"

import { api } from "@/lib/api"
import type { SystemConfig } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
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

interface ConfigListResponse {
  data: SystemConfig[]
  records: number
}

interface FormState {
  id: string
  key: string
  value: string
  description: string
}

const emptyForm: FormState = { id: "", key: "", value: "", description: "" }

export default function SystemConfigPage() {
  const qc = useQueryClient()
  const [search, setSearch] = useState("")
  const [editOpen, setEditOpen] = useState(false)
  const [form, setForm] = useState<FormState>(emptyForm)

  const { data, isLoading } = useQuery<ConfigListResponse>({
    queryKey: ["system-config"],
    queryFn: () =>
      api.get<ConfigListResponse>("/api/core/system/config/query", {
        pageIndex: 1,
        pageSize: 200,
      }),
  })

  const all = useMemo(() => data?.data ?? [], [data])
  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    if (!q) return all
    return all.filter(
      (c) =>
        c.key.toLowerCase().includes(q) ||
        c.value.toLowerCase().includes(q) ||
        (c.description || "").toLowerCase().includes(q),
    )
  }, [all, search])

  const addMutation = useMutation({
    mutationFn: (body: { key: string; value: string; description: string }) =>
      api.post("/api/core/system/config/add", body),
    onSuccess: () => {
      toast.success("已新增")
      setEditOpen(false)
      qc.invalidateQueries({ queryKey: ["system-config"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const updateMutation = useMutation({
    mutationFn: (body: FormState) => api.put("/api/core/system/config/update", body),
    onSuccess: () => {
      toast.success("已更新")
      setEditOpen(false)
      qc.invalidateQueries({ queryKey: ["system-config"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.del("/api/core/system/config/delete", { id }),
    onSuccess: () => {
      toast.success("已删除")
      qc.invalidateQueries({ queryKey: ["system-config"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  function submit() {
    if (!form.key.trim()) {
      toast.error("请填写 key")
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
        <h1 className="text-2xl font-bold">系统配置</h1>
        <Button
          size="sm"
          onClick={() => {
            setForm({ ...emptyForm })
            setEditOpen(true)
          }}
        >
          <Plus /> 新增配置
        </Button>
      </div>

      <div className="flex flex-wrap items-end gap-2">
        <div className="space-y-1">
          <Label className="text-xs">关键字</Label>
          <Input
            className="h-8 w-64"
            placeholder="key / value / 描述"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <Button size="sm" variant="outline" onClick={() => setSearch("")}>
          <Search /> 重置
        </Button>
        <span className="text-sm text-muted-foreground">
          共 {filtered.length} / {all.length} 条
        </span>
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Key</TableHead>
              <TableHead>Value</TableHead>
              <TableHead>描述</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={4} className="py-8 text-center text-muted-foreground">
                  加载中…
                </TableCell>
              </TableRow>
            ) : filtered.length === 0 ? (
              <TableRow>
                <TableCell colSpan={4} className="py-8 text-center text-muted-foreground">
                  暂无数据
                </TableCell>
              </TableRow>
            ) : (
              filtered.map((c) => (
                <TableRow key={c.id}>
                  <TableCell className="font-mono text-xs font-medium">{c.key}</TableCell>
                  <TableCell className="max-w-[280px] truncate font-mono text-xs" title={c.value}>
                    {c.value}
                  </TableCell>
                  <TableCell className="text-muted-foreground">{c.description || "-"}</TableCell>
                  <TableCell className="text-right">
                    <div className="inline-flex gap-1">
                      <Button
                        size="icon"
                        variant="ghost"
                        title="编辑"
                        onClick={() => {
                          setForm({
                            id: c.id,
                            key: c.key,
                            value: c.value,
                            description: c.description || "",
                          })
                          setEditOpen(true)
                        }}
                      >
                        <Pencil />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        title="删除"
                        onClick={() => {
                          if (window.confirm(`确认删除配置 ${c.key}？`))
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

      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{form.id ? "编辑配置" : "新增配置"}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1">
              <Label>Key</Label>
              <Input
                className="font-mono text-xs"
                disabled={!!form.id}
                value={form.key}
                onChange={(e) => setForm({ ...form, key: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>Value</Label>
              <Textarea
                rows={6}
                className="font-mono text-xs"
                value={form.value}
                onChange={(e) => setForm({ ...form, value: e.target.value })}
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
            <Button onClick={submit} disabled={addMutation.isPending || updateMutation.isPending}>
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
