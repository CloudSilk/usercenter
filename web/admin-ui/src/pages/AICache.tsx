import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Trash2, RefreshCw, Database, SlidersHorizontal, Plus } from "lucide-react"
import { toast } from "sonner"

import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

interface CacheStats {
  hits: number
  misses: number
  hitRate: number
  entries: number
}

interface BucketStats {
  tokens: number
  rate: number
  burst: number
}

// 后端 /ratelimit/stats 返回 { buckets: { [principalID]: BucketStats } }
interface RateLimitResp {
  buckets?: Record<string, BucketStats>
}

interface RateForm {
  principal: string
  ratePerSecond: number
  burst: number
}

const emptyForm: RateForm = { principal: "", ratePerSecond: 5, burst: 10 }

export default function AICache() {
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery<{ data: CacheStats }>({
    queryKey: ["ai-cache-stats"],
    queryFn: () => api.get("/admin/api/ai-cache/stats"),
    refetchInterval: 10000,
  })

  const { data: ratelimitData } = useQuery<{ data: RateLimitResp }>({
    queryKey: ["ratelimit-stats"],
    queryFn: () => api.get("/admin/api/ratelimit/stats"),
    refetchInterval: 10000,
  })

  const stats = data?.data
  const ratelimit = ratelimitData?.data

  const [form, setForm] = useState<RateForm>(emptyForm)
  const [open, setOpen] = useState(false)

  async function clearCache() {
    if (!confirm("确认清空所有缓存条目？")) return
    try {
      await api.del("/admin/api/ai-cache")
      toast.success("缓存已清空")
      queryClient.invalidateQueries({ queryKey: ["ai-cache-stats"] })
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : "清空失败")
    }
  }

  // 设置/调整某 principal 的令牌桶速率与突发容量，对应 PUT /admin/api/ratelimit/:principal。
  const setLimitMut = useMutation({
    mutationFn: async (f: RateForm) =>
      api.put(`/admin/api/ratelimit/${encodeURIComponent(f.principal)}`, {
        ratePerSecond: f.ratePerSecond,
        burst: f.burst,
      }),
    onSuccess: () => {
      toast.success("限流已更新")
      setOpen(false)
      queryClient.invalidateQueries({ queryKey: ["ratelimit-stats"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const buckets = ratelimit?.buckets ?? {}
  const bucketEntries = Object.entries(buckets)

  return (
    <div className="space-y-6 p-4 md:p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">AI 缓存与限流</h1>
          <p className="text-sm text-muted-foreground">
            语义缓存统计 + 每用户令牌桶限流状态
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={() => queryClient.invalidateQueries()}>
          <RefreshCw className="h-4 w-4" /> 刷新
        </Button>
      </div>

      {/* 缓存统计 */}
      <div className="space-y-3">
        <div className="flex items-center gap-2">
          <Database className="h-5 w-5" />
          <h2 className="text-lg font-semibold">语义缓存</h2>
        </div>

        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          <div className="rounded-lg border p-4">
            <p className="text-xs text-muted-foreground">缓存条目</p>
            <p className="text-2xl font-bold">{stats?.entries ?? 0}</p>
          </div>
          <div className="rounded-lg border p-4">
            <p className="text-xs text-muted-foreground">命中次数</p>
            <p className="text-2xl font-bold text-green-600">{stats?.hits ?? 0}</p>
          </div>
          <div className="rounded-lg border p-4">
            <p className="text-xs text-muted-foreground">未命中</p>
            <p className="text-2xl font-bold text-orange-600">{stats?.misses ?? 0}</p>
          </div>
          <div className="rounded-lg border p-4">
            <p className="text-xs text-muted-foreground">命中率</p>
            <p className="text-2xl font-bold text-blue-600">
              {stats ? stats.hitRate.toFixed(1) : "0.0"}%
            </p>
          </div>
        </div>

        {isLoading ? (
          <p className="text-sm text-muted-foreground">加载中…</p>
        ) : null}

        <Button variant="destructive" size="sm" onClick={clearCache}>
          <Trash2 className="h-4 w-4" /> 清空缓存
        </Button>
      </div>

      {/* 限流状态 */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold">限流桶状态</h2>
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              setForm(emptyForm)
              setOpen(true)
            }}
          >
            <Plus /> 设置限流
          </Button>
        </div>
        {bucketEntries.length === 0 ? (
          <p className="rounded-lg border p-4 text-center text-sm text-muted-foreground">
            暂无活跃的限流桶
          </p>
        ) : (
          <div className="rounded-md border">
            <table className="w-full text-sm">
              <thead className="border-b bg-muted/50">
                <tr>
                  <th className="px-3 py-2 text-left">用户 ID</th>
                  <th className="px-3 py-2 text-left">剩余令牌</th>
                  <th className="px-3 py-2 text-left">速率 (req/s)</th>
                  <th className="px-3 py-2 text-left">桶容量</th>
                  <th className="px-3 py-2 text-right">操作</th>
                </tr>
              </thead>
              <tbody>
                {bucketEntries.map(([id, b]) => (
                  <tr key={id} className="border-b last:border-0">
                    <td className="px-3 py-2 font-mono text-xs">{id}</td>
                    <td className="px-3 py-2">{(b.tokens ?? 0).toFixed(1)}</td>
                    <td className="px-3 py-2">{b.rate ?? 0}</td>
                    <td className="px-3 py-2">{b.burst ?? 0}</td>
                    <td className="px-3 py-2 text-right">
                      <Button
                        size="icon"
                        variant="ghost"
                        title="调整限流"
                        onClick={() => {
                          setForm({
                            principal: id,
                            ratePerSecond: b.rate ?? 1,
                            burst: b.burst ?? 1,
                          })
                          setOpen(true)
                        }}
                      >
                        <SlidersHorizontal className="h-4 w-4" />
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* 限流配置弹窗：新建或调整 principal 的令牌桶 */}
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>设置限流</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1">
              <Label className="text-xs">Principal ID（用户 / Agent / API Key）</Label>
              <Input
                value={form.principal}
                placeholder="user-xxx / agent-yyy"
                onChange={(e) => setForm({ ...form, principal: e.target.value })}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label className="text-xs">速率（令牌/秒）</Label>
                <Input
                  type="number"
                  min={1}
                  step={1}
                  value={form.ratePerSecond}
                  onChange={(e) =>
                    setForm({ ...form, ratePerSecond: Number(e.target.value) })
                  }
                />
              </div>
              <div className="space-y-1">
                <Label className="text-xs">桶容量（突发上限）</Label>
                <Input
                  type="number"
                  min={1}
                  step={1}
                  value={form.burst}
                  onChange={(e) => setForm({ ...form, burst: Number(e.target.value) })}
                />
              </div>
            </div>
            <p className="text-xs text-muted-foreground">
              保存后令牌桶立即重置为满容量，新限流即刻生效。
            </p>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              取消
            </Button>
            <Button
              onClick={() => {
                if (!form.principal.trim()) {
                  toast.error("Principal ID 不能为空")
                  return
                }
                if (form.ratePerSecond <= 0 || form.burst <= 0) {
                  toast.error("速率与桶容量必须为正数")
                  return
                }
                setLimitMut.mutate(form)
              }}
              disabled={setLimitMut.isPending}
            >
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
