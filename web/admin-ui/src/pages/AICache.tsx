import { useQuery, useQueryClient } from "@tanstack/react-query"
import { Trash2, RefreshCw, Database } from "lucide-react"

import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { toast } from "sonner"

interface CacheStats {
  hits: number
  misses: number
  hitRate: number
  entries: number
}

export default function AICache() {
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery<{ data: CacheStats }>({
    queryKey: ["ai-cache-stats"],
    queryFn: () => api.get("/admin/api/ai-cache/stats"),
    refetchInterval: 10000,
  })

  const { data: ratelimitData } = useQuery<{ data: { buckets?: Record<string, any> } }>({
    queryKey: ["ratelimit-stats"],
    queryFn: () => api.get("/admin/api/ratelimit/stats"),
    refetchInterval: 10000,
  })

  const stats = data?.data
  const ratelimit = ratelimitData?.data

  async function clearCache() {
    if (!confirm("确认清空所有缓存条目？")) return
    try {
      await api.del("/admin/api/ai-cache")
      toast.success("缓存已清空")
      queryClient.invalidateQueries({ queryKey: ["ai-cache-stats"] })
    } catch (e: any) {
      toast.error(e.message || "清空失败")
    }
  }

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
        <h2 className="text-lg font-semibold">限流桶状态</h2>
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
                </tr>
              </thead>
              <tbody>
                {bucketEntries.map(([id, b]: [string, any]) => (
                  <tr key={id} className="border-b last:border-0">
                    <td className="px-3 py-2 font-mono text-xs">{id}</td>
                    <td className="px-3 py-2">{(b.tokens ?? 0).toFixed(1)}</td>
                    <td className="px-3 py-2">{b.rate ?? 0}</td>
                    <td className="px-3 py-2">{b.burst ?? 0}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
