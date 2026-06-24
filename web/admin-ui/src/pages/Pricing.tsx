import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Pencil, Plus, Trash2 } from "lucide-react"

import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
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

interface Price {
  id: string
  tenantID: string
  modelName: string
  inputPer1M: number
  outputPer1M: number
}

const empty: Omit<Price, "id"> = {
  tenantID: "",
  modelName: "",
  inputPer1M: 0,
  outputPer1M: 0,
}

interface ListResp {
  data: Price[]
}

export default function Pricing() {
  const qc = useQueryClient()
  const { data, isLoading } = useQuery<Price[]>({
    queryKey: ["pricing"],
    queryFn: async () => {
      const res = await api.get<ListResp>("/admin/api/pricing")
      return res.data ?? []
    },
  })

  const [form, setForm] = useState<Omit<Price, "id"> & { id: string }>({ ...empty, id: "" })
  const [open, setOpen] = useState(false)

  const saveMut = useMutation({
    mutationFn: async (p: Omit<Price, "id"> & { id: string }) => {
      if (p.id) {
        return api.put(`/admin/api/pricing/${p.id}`, p)
      }
      const { id: _id, ...rest } = p
      void _id
      return api.post("/admin/api/pricing", rest)
    },
    onSuccess: () => {
      toast.success("已保存")
      setOpen(false)
      qc.invalidateQueries({ queryKey: ["pricing"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const delMut = useMutation({
    mutationFn: (id: string) => api.del(`/admin/api/pricing/${id}`),
    onSuccess: () => {
      toast.success("已删除")
      qc.invalidateQueries({ queryKey: ["pricing"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const prices = data ?? []

  return (
    <div className="space-y-4 p-4 md:p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">计价表</h1>
          <p className="text-sm text-muted-foreground">
            按模型设置每百万 token 的输入/输出单价（USD）
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
              <TableHead>模型</TableHead>
              <TableHead>租户（空=全局）</TableHead>
              <TableHead>输入 / 1M token</TableHead>
              <TableHead>输出 / 1M token</TableHead>
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
            ) : prices.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="py-8 text-center text-muted-foreground">
                  暂无计价配置
                </TableCell>
              </TableRow>
            ) : (
              prices.map((p) => (
                <TableRow key={p.id}>
                  <TableCell className="font-medium">{p.modelName}</TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">
                    {p.tenantID || "全局"}
                  </TableCell>
                  <TableCell>${p.inputPer1M}</TableCell>
                  <TableCell>${p.outputPer1M}</TableCell>
                  <TableCell className="text-right">
                    <Button
                      size="icon"
                      variant="ghost"
                      onClick={() => {
                        setForm({ ...p })
                        setOpen(true)
                      }}
                    >
                      <Pencil />
                    </Button>
                    <Button
                      size="icon"
                      variant="ghost"
                      onClick={() => {
                        if (confirm("确认删除此计价？")) delMut.mutate(p.id)
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
            <DialogTitle>{form.id ? "编辑计价" : "新增计价"}</DialogTitle>
          </DialogHeader>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <Label className="text-xs">模型名称</Label>
              <Input
                value={form.modelName}
                placeholder="gpt-4"
                onChange={(e) => setForm({ ...form, modelName: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">租户 ID（空=全局）</Label>
              <Input
                value={form.tenantID}
                onChange={(e) => setForm({ ...form, tenantID: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">输入单价 / 1M token</Label>
              <Input
                type="number"
                step="0.01"
                value={form.inputPer1M}
                onChange={(e) => setForm({ ...form, inputPer1M: Number(e.target.value) })}
              />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">输出单价 / 1M token</Label>
              <Input
                type="number"
                step="0.01"
                value={form.outputPer1M}
                onChange={(e) => setForm({ ...form, outputPer1M: Number(e.target.value) })}
              />
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
