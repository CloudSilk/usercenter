import { useEffect, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { RotateCcw, Save, Wallet } from "lucide-react"

import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

interface LoopStatus {
  running: boolean
  intervalSeconds: number
  scanAgeMinutes: number
  batchSize: number
  alertAgeHours: number
  billRetentionDays: number
  lastReconcileAt?: string
  lastReconcileOK: boolean
  lastProcessed: number
  lastDailyReport?: string
  lastBillDownload?: string
}

interface StatsConfigUpdate {
  intervalSeconds?: number
  scanAgeMinutes?: number
  batchSize?: number
  alertAgeHours?: number
  alertSilenceMinutes?: number
  billRetentionDays?: number
}

const FIELDS: [keyof StatsConfigUpdate, string, number][] = [
  ["intervalSeconds", "轮询间隔(秒)", 10],
  ["scanAgeMinutes", "扫描窗口(分钟)", 1],
  ["batchSize", "单轮批次上限", 1],
  ["alertAgeHours", "滞留告警阈值(小时)", 1],
  ["alertSilenceMinutes", "告警静默窗口(分钟)", 1],
  ["billRetentionDays", "账单保留期(天)", 7],
]

/** 微信支付设置页:对账参数运行时配置(即时生效,持久化后重启仍优先于 Nacos 基线)。 */
export default function WechatPaySettings() {
  const qc = useQueryClient()
  const loopStatus = useQuery({
    queryKey: ["pay-loop-status"],
    queryFn: () => api.get<LoopStatus>("/api/core/wechat/pay/stats/loop-status"),
  })
  const [settings, setSettings] = useState<StatsConfigUpdate>({})
  const [loaded, setLoaded] = useState(false)

  // loop-status 首次到达后回填当前运行值
  useEffect(() => {
    const s = loopStatus.data
    if (s && !loaded) {
      setSettings({
        intervalSeconds: s.intervalSeconds,
        scanAgeMinutes: s.scanAgeMinutes,
        batchSize: s.batchSize,
        alertAgeHours: s.alertAgeHours,
        billRetentionDays: s.billRetentionDays,
      })
      setLoaded(true)
    }
  }, [loopStatus.data, loaded])

  const saveSettings = useMutation({
    mutationFn: (v: StatsConfigUpdate) => api.put("/api/core/wechat/pay/stats/config", v),
    onSuccess: () => {
      toast.success("对账参数已更新")
      qc.invalidateQueries({ queryKey: ["pay-loop-status"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })
  const resetConfig = useMutation({
    mutationFn: () => api.del("/api/core/wechat/pay/stats/config"),
    onSuccess: () => {
      toast.success("已恢复 Nacos 基线")
      qc.invalidateQueries({ queryKey: ["pay-loop-status"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })
  const reconcileNow = useMutation({
    mutationFn: () =>
      api.post<{ processed: number }>("/api/core/wechat/pay/stats/reconcile-now"),
    onSuccess: (r) => {
      toast.success(`对账完成:本轮处理 ${r.processed} 笔`)
      qc.invalidateQueries({ queryKey: ["pay-loop-status"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  return (
    <div className="max-w-2xl space-y-6 p-6">
      <div className="flex items-center gap-2">
        <Wallet className="h-5 w-5" />
        <h1 className="text-xl font-semibold">微信支付设置</h1>
      </div>

      {/* 运行状态摘要 */}
      {loopStatus.data && (
        <div className="rounded border bg-muted/30 px-4 py-2 text-sm">
          <span className="flex items-center gap-1">
            <span
              className={`inline-block h-2 w-2 rounded-full ${loopStatus.data.running ? "bg-green-500" : "bg-red-500"}`}
            />
            对账循环 {loopStatus.data.running ? "运行中" : "已停止"}
          </span>
          <span className="text-muted-foreground">
            最近对账{" "}
            {loopStatus.data.lastReconcileAt
              ? new Date(loopStatus.data.lastReconcileAt).toLocaleString()
              : "尚未执行"}{" "}
            · 处理 {loopStatus.data.lastProcessed} 笔 · 日报 {loopStatus.data.lastDailyReport || "未推送"} ·
            账单 {loopStatus.data.lastBillDownload || "未下载"}
          </span>
        </div>
      )}

      {/* 对账参数表单 */}
      <section className="space-y-3 rounded border p-4">
        <h2 className="font-medium">对账参数(运行时生效)</h2>
        <div className="grid grid-cols-2 gap-3">
          {FIELDS.map(([key, label, min]) => (
            <div key={key} className="space-y-1">
              <Label>{label}</Label>
              <Input
                type="number"
                min={min}
                data-testid={`setting-${key}`}
                value={settings[key] ?? ""}
                onChange={(e) =>
                  setSettings({ ...settings, [key]: Number(e.target.value) || undefined })
                }
              />
            </div>
          ))}
        </div>
        <p className="text-xs text-muted-foreground">
          参数即时生效,保存后持久化(重启仍优先于 Nacos 基线);轮询间隔最小 10s,批次上限 1000,账单保留期最小 7 天。
        </p>
        <div className="flex flex-wrap gap-2">
          <Button
            disabled={saveSettings.isPending}
            onClick={() => saveSettings.mutate(settings)}
          >
            <Save className="mr-1 h-4 w-4" /> 保存
          </Button>
          <Button
            variant="outline"
            disabled={resetConfig.isPending}
            onClick={() => resetConfig.mutate()}
          >
            <RotateCcw className="mr-1 h-4 w-4" /> 恢复 Nacos 基线
          </Button>
          <Button
            variant="outline"
            disabled={reconcileNow.isPending}
            onClick={() => reconcileNow.mutate()}
          >
            {reconcileNow.isPending ? "对账中..." : "立即对账"}
          </Button>
        </div>
      </section>
    </div>
  )
}
