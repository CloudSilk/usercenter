import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Pencil, Plus, Trash2 } from "lucide-react"

import { api } from "@/lib/api"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
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

interface Budget {
  id: string
  tenantID: string
  principalID: string
  modelName: string
  dailyTokenLimit: number
  dailyCostLimit: number
  monthlyTokenLimit: number
  monthlyCostLimit: number
  enable: boolean
}

const empty: Omit<Budget, "id"> = {
  tenantID: "",
  principalID: "",
  modelName: "",
  dailyTokenLimit: 0,
  dailyCostLimit: 0,
  monthlyTokenLimit: 0,
  monthlyCostLimit: 0,
  enable: true,
}

interface ListResp {
  data: Budget[]
}

export default function Budgets() {
  const qc = useQueryClient()
  const { data, isLoading } = useQuery<Budget[]>({
    queryKey: ["budgets"],
    queryFn: async () => {
      const res = await api.get<ListResp>("/admin/api/usage/budgets")
      return res.data ?? []
    },
  })

  const [form, setForm] = useState<Omit<Budget, "id"> & { id: string }>({ ...empty, id: "" })
  const [open, setOpen] = useState(false)

  const saveMut = useMutation({
    mutationFn: async (b: Omit<Budget, "id"> & { id: string }) => {
      if (b.id) {
        return api.put(`/admin/api/usage/budgets/${b.id}`, b)
      }
      const { id: _id, ...rest } = b
      void _id
      return api.post("/admin/api/usage/budgets", rest)
    },
    onSuccess: () => {
      toast.success("已保存")
      setOpen(false)
      qc.invalidateQueries({ queryKey: ["budgets"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const delMut = useMutation({
    mutationFn: (id: string) => api.del(`/admin/api/usage/budgets/${id}`),
    onSuccess: () => {
      toast.success("已删除")
      qc.invalidateQueries({ queryKey: ["budgets"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const budgets = data ?? []

  return (
    <div className="space-y-4 p-4 md:p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">预算配额</h1>
          <p className="text-sm text-muted-foreground">
            按租户/用户/模型设置每日与每月的 token 及费用上限
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
              <TableHead>租户</TableHead>
              <TableHead>用户</TableHead>
              <TableHead>模型</TableHead>
              <TableHead>日 Token</TableHead>
              <TableHead>日费用</TableHead>
              <TableHead>月 Token</TableHead>
              <TableHead>月费用</TableHead>
              <TableHead>状态</TableHead>
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
            ) : budgets.length === 0 ? (
              <TableRow>
                <TableCell colSpan={9} className="py-8 text-center text-muted-foreground">
                  暂无预算配置
                </TableCell>
              </TableRow>
            ) : (
              budgets.map((b) => (
                <TableRow key={b.id}>
                  <TableCell className="max-w-[100px] truncate font-mono text-xs" title={b.tenantID}>
                    {b.tenantID || "全部"}
                  </TableCell>
                  <TableCell className="max-w-[100px] truncate font-mono text-xs" title={b.principalID}>
                    {b.principalID || "全部"}
                  </TableCell>
                  <TableCell>{b.modelName || "全部"}</TableCell>
                  <TableCell>{b.dailyTokenLimit || "-"}</TableCell>
                  <TableCell>${b.dailyCostLimit || 0}</TableCell>
                  <TableCell>{b.monthlyTokenLimit || "-"}</TableCell>
                  <TableCell>${b.monthlyCostLimit || 0}</TableCell>
                  <TableCell>
                    {b.enable ? <Badge>启用</Badge> : <Badge variant="secondary">禁用</Badge>}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      size="icon"
                      variant="ghost"
                      onClick={() => {
                        setForm({ ...b })
                        setOpen(true)
                      }}
                    >
                      <Pencil />
                    </Button>
                    <Button
                      size="icon"
                      variant="ghost"
                      onClick={() => {
                        if (confirm("确认删除此预算？")) delMut.mutate(b.id)
                      }}
                    >
                      <Trash2 className="text-destructive" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{form.id ? "编辑预算" : "新增预算"}</DialogTitle>
          </DialogHeader>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <Label className="text-xs">租户 ID（空=全部）</Label>
              <Input
                value={form.tenantID}
                onChange={(e) => setForm({ ...form, tenantID: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">用户 ID（空=租户级）</Label>
              <Input
                value={form.principalID}
                onChange={(e) => setForm({ ...form, principalID: e.target.value })}
              />
            </div>
            <div className="col-span-2 space-y-1">
              <Label className="text-xs">模型（空=全部）</Label>
              <Input
                value={form.modelName}
                onChange={(e) => setForm({ ...form, modelName: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">日 Token 上限</Label>
              <Input
                type="number"
                value={form.dailyTokenLimit}
                onChange={(e) => setForm({ ...form, dailyTokenLimit: Number(e.target.value) })}
              />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">日费用上限 (USD)</Label>
              <Input
                type="number"
                step="0.01"
                value={form.dailyCostLimit}
                onChange={(e) => setForm({ ...form, dailyCostLimit: Number(e.target.value) })}
              />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">月 Token 上限</Label>
              <Input
                type="number"
                value={form.monthlyTokenLimit}
                onChange={(e) => setForm({ ...form, monthlyTokenLimit: Number(e.target.value) })}
              />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">月费用上限 (USD)</Label>
              <Input
                type="number"
                step="0.01"
                value={form.monthlyCostLimit}
                onChange={(e) => setForm({ ...form, monthlyCostLimit: Number(e.target.value) })}
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
            <Button variant="outline" onClick={() => setOpen(false)}>
              取消
            </Button>
            <Button onClick={() => saveMut.mutate(form)} disabled={saveMut.isPending}>
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
