import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Download, Pencil, Plus, Settings, Trash2, Wallet, XCircle } from "lucide-react"
import {
  Bar,
  BarChart,
  ResponsiveContainer,
  Tooltip as ChartTooltip,
  XAxis,
  YAxis,
} from "recharts"

import { api, getToken } from "@/lib/api"
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

const CFG = "/api/core/wechat/pay/config"
const ORDER = "/api/core/wechat/pay/order"
const REFUND = "/api/core/wechat/pay/refund"
const BILL = "/api/core/wechat/pay/bill"

interface BillFileItem {
  id: string
  tenantID: string
  configID: string
  billDate: string
  billType: string
}

interface RefundReasonTrendPoint {
  month: string
  reasonCode: string
  count: number
  amount: number
}

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

interface BatchCloseFailureItem {
  outTradeNo: string
  error: string
}

interface StatsConfigUpdate {
  intervalSeconds?: number
  scanAgeMinutes?: number
  batchSize?: number
  alertAgeHours?: number
  alertSilenceMinutes?: number
  billRetentionDays?: number
}

interface PayConfigInfo {
  id?: string
  tenantID: string
  wechatConfigID: string
  appID: string
  mchID: string
  mchSerialNo: string
  apiV3Key?: string
  privateKey?: string
  notifyURL?: string
  refundNotifyURL?: string
  refundApprovalRequired?: boolean
  enable?: boolean
  description?: string
}

interface PayOrderItem {
  id: string
  tenantID: string
  userID: string
  mchID: string
  outTradeNo: string
  transactionID?: string
  description?: string
  amount: number
  status: string
  tradeState?: string
  createdAt: string
  paidAt?: string
}

interface RefundItem {
  id: string
  outTradeNo: string
  outRefundNo: string
  amount: number
  reasonCode?: string
  reason?: string
  status: string
  approverID?: string
  approveComment?: string
  approvedAt?: string
}

interface ListResp<T> {
  data?: T[]
  records?: number
  pages?: number
  total?: number
}

const STATUS_BADGE: Record<string, string> = {
  CREATED: "bg-blue-500/15 text-blue-600",
  PAID: "bg-green-500/15 text-green-600",
  CLOSED: "bg-gray-500/15 text-gray-600",
  PENDING: "bg-amber-500/15 text-amber-600",
  PROCESSING: "bg-blue-500/15 text-blue-600",
  SUCCESS: "bg-green-500/15 text-green-600",
  REJECTED: "bg-red-500/15 text-red-600",
  ABNORMAL: "bg-red-500/15 text-red-600",
}

/** 带 token 的 CSV/blob 下载(api 客户端仅处理 JSON)。
 *  后端出错时会返回 JSON 载荷,此处识别并弹 toast 而非保存错误内容。 */
async function downloadBlob(url: string, fallbackName: string): Promise<void> {
  const token = getToken()
  const res = await fetch(url, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  })
  const contentType = res.headers.get("Content-Type") || ""
  if (!res.ok || contentType.includes("application/json")) {
    let message = `下载失败 HTTP ${res.status}`
    try {
      const body = (await res.json()) as { message?: string }
      if (body.message) message = body.message
    } catch {
      /* 非 JSON 响应,保留默认错误 */
    }
    toast.error(message)
    return
  }
  const blob = await res.blob()
  const cd = res.headers.get("Content-Disposition") || ""
  const m = /filename="?([^";]+)"?/.exec(cd)
  const a = document.createElement("a")
  a.href = URL.createObjectURL(blob)
  a.download = m ? m[1] : fallbackName
  a.click()
  URL.revokeObjectURL(a.href)
}

/** 本地时区的 YYYY-MM-DD。 */
function fmtLocalDate(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

const emptyConfig: PayConfigInfo = {
  tenantID: "",
  wechatConfigID: "",
  appID: "",
  mchID: "",
  mchSerialNo: "",
  apiV3Key: "",
  privateKey: "",
  notifyURL: "",
  refundNotifyURL: "",
  refundApprovalRequired: false,
  enable: true,
  description: "",
}

function StatusBadge({ status }: { status: string }) {
  return (
    <Badge className={STATUS_BADGE[status] || ""} variant="secondary">
      {status}
    </Badge>
  )
}

export default function WechatPay() {
  const qc = useQueryClient()

  // ---- 商户配置 ----
  const [editing, setEditing] = useState<PayConfigInfo | null>(null)
  const [isNew, setIsNew] = useState(false)

  const configs = useQuery({
    queryKey: ["pay-configs"],
    queryFn: () => api.get<ListResp<PayConfigInfo>>(`${CFG}/query`),
  })

  const saveConfig = useMutation({
    mutationFn: (c: PayConfigInfo) =>
      c.id
        ? api.put(`${CFG}/update`, c)
        : api.post(`${CFG}/add`, c),
    onSuccess: () => {
      toast.success("已保存")
      setEditing(null)
      qc.invalidateQueries({ queryKey: ["pay-configs"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const deleteConfig = useMutation({
    mutationFn: (id: string) => api.del(`${CFG}/delete?id=${id}`),
    onSuccess: () => {
      toast.success("已删除")
      qc.invalidateQueries({ queryKey: ["pay-configs"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  // ---- 订单 ----
  const [orderStatus, setOrderStatus] = useState("PAID")
  const [orderPage, setOrderPage] = useState(1)
  const orderPageSize = 20
  const [closeFailures, setCloseFailures] = useState<BatchCloseFailureItem[]>([])
  const [selectedOrders, setSelectedOrders] = useState<Set<string>>(new Set())
  const orders = useQuery({
    queryKey: ["pay-orders", orderStatus, orderPage],
    queryFn: () =>
      api.get<ListResp<PayOrderItem>>(`${ORDER}/query`, {
        status: orderStatus || undefined,
        pageIndex: orderPage,
        pageSize: orderPageSize,
      }),
  })

  const batchClose = useMutation({
    mutationFn: (outTradeNos: string[]) =>
      api.post<{ closed: number; skipped: number; failures?: { outTradeNo: string; error: string }[] }>(
        `${ORDER}/batch-close`,
        { outTradeNos },
      ),
    onSuccess: (r) => {
      toast.success(`批量关单完成:关闭 ${r.closed},跳过 ${r.skipped}`)
      setCloseFailures(r.failures || [])
      setSelectedOrders(new Set())
      qc.invalidateQueries({ queryKey: ["pay-orders"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  // ---- 对账循环状态 ----
  const loopStatus = useQuery({
    queryKey: ["pay-loop-status"],
    queryFn: () => api.get<LoopStatus>("/api/core/wechat/pay/stats/loop-status"),
    refetchInterval: 30_000,
  })
  const reconcileNow = useMutation({
    mutationFn: () =>
      api.post<{ processed: number }>("/api/core/wechat/pay/stats/reconcile-now"),
    onSuccess: (r) => {
      toast.success(`对账完成:本轮处理 ${r.processed} 笔`)
      qc.invalidateQueries({ queryKey: ["pay-loop-status"] })
      qc.invalidateQueries({ queryKey: ["pay-orders"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  function toggleOrder(no: string) {
    setSelectedOrders((prev) => {
      const next = new Set(prev)
      if (next.has(no)) next.delete(no)
      else next.add(no)
      return next
    })
  }

  // ---- 退款原因趋势(近6个月) ----
  const trend = useQuery({
    queryKey: ["pay-reason-trend"],
    queryFn: () =>
      api.get<RefundReasonTrendPoint[]>(
        "/api/core/wechat/pay/stats/refund-reason/trend",
        { months: 6 },
      ),
  })

  // ---- 账单归档 ----
  const [billConfigID, setBillConfigID] = useState("")
  const [billConfigIDPull, setBillConfigIDPull] = useState("")
  const [billDate, setBillDate] = useState(fmtLocalDate(new Date(Date.now() - 86400000)))
  const [billType, setBillType] = useState("ALL")
  const [pulling, setPulling] = useState(false)
  const [billDays, setBillDays] = useState(30)
  const [billPage, setBillPage] = useState(1)
  const billPageSize = 10
  // 服务端分页:pageIndex/pageSize 传给后端,pages 由响应返回
  const bills = useQuery({
    queryKey: ["pay-bills", billConfigID, billDays, billPage],
    queryFn: () =>
      api.get<ListResp<BillFileItem>>(`${BILL}/list`, {
        configID: billConfigID || undefined,
        days: billDays,
        pageIndex: billPage,
        pageSize: billPageSize,
      }),
  })
  const billRows = bills.data?.data || []
  const billTotalPages = Math.max(1, bills.data?.pages ?? 1)

  /** 手动拉取指定日期/类型的交易账单(实时调微信侧)。 */
  async function pullTradeBill() {
    if (!billConfigIDPull || !billDate) {
      toast.error("请先填写商户配置ID")
      return
    }
    setPulling(true)
    try {
      await downloadBlob(
        `/api/core/wechat/pay/order/trade-bill?configID=${encodeURIComponent(billConfigIDPull)}` +
          `&billDate=${billDate}&billType=${billType}`,
        `trade_bill_${billDate}.csv`,
      )
    } finally {
      setPulling(false)
      qc.invalidateQueries({ queryKey: ["pay-bills"] })
    }
  }

  // ---- 退款审核 ----
  const [approving, setApproving] = useState<{ refund: RefundItem; approved: boolean } | null>(null)
  const [approveComment, setApproveComment] = useState("")
  const [refundStatus, setRefundStatus] = useState("PENDING")
  const pendingRefunds = useQuery({
    queryKey: ["pay-refunds-pending", refundStatus],
    queryFn: () =>
      api.get<ListResp<RefundItem>>(`${REFUND}/query`, {
        status: refundStatus || undefined,
      }),
  })
  const approveRefund = useMutation({
    mutationFn: (v: { outRefundNo: string; approved: boolean; comment: string }) =>
      api.post(`${REFUND}/approve`, v),
    onSuccess: (_r, v) => {
      toast.success(v.approved ? "已通过并提交微信" : "已拒绝")
      setApproving(null)
      setApproveComment("")
      qc.invalidateQueries({ queryKey: ["pay-refunds-pending"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  function openApprove(r: RefundItem, approved: boolean) {
    setApproveComment("")
    setApproving({ refund: r, approved })
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center gap-2">
        <Wallet className="h-5 w-5" />
        <h1 className="text-xl font-semibold">微信支付</h1>
      </div>

      {/* 对账循环状态 */}
      {loopStatus.data && (
        <div className="flex flex-wrap items-center gap-x-6 gap-y-1 rounded border bg-muted/30 px-4 py-2 text-sm">
          <span className="flex items-center gap-1">
            <span
              className={`inline-block h-2 w-2 rounded-full ${loopStatus.data.running ? "bg-green-500" : "bg-red-500"}`}
            />
            对账循环 {loopStatus.data.running ? "运行中" : "已停止"}
          </span>
          <span className="text-muted-foreground">
            间隔 {loopStatus.data.intervalSeconds}s · 扫描窗口 {loopStatus.data.scanAgeMinutes}m ·
            批次 {loopStatus.data.batchSize} · 账单保留 {loopStatus.data.billRetentionDays} 天
          </span>
          {loopStatus.data.lastReconcileAt && (
            <span className="text-muted-foreground">
              最近对账{" "}
              {new Date(loopStatus.data.lastReconcileAt).toLocaleString()}(
              {loopStatus.data.lastProcessed} 笔
              {loopStatus.data.lastReconcileOK ? "" : ",异常"})
            </span>
          )}
          {loopStatus.data.lastDailyReport && (
            <span className="text-muted-foreground">
              日报 {loopStatus.data.lastDailyReport}
            </span>
          )}
          <Button
            size="sm"
            variant="outline"
            disabled={reconcileNow.isPending}
            onClick={() => reconcileNow.mutate()}
          >
            {reconcileNow.isPending ? "对账中..." : "立即对账"}
          </Button>
          <Button size="sm" variant="outline" asChild>
            <a href="/web/admin/wechatpay/settings">
              <Settings className="mr-1 h-4 w-4" /> 设置
            </a>
          </Button>
        </div>
      )}

      {/* 商户配置 */}
      <section className="space-y-2">
        <div className="flex items-center justify-between">
          <h2 className="font-medium">商户配置</h2>
          <Button
            size="sm"
            onClick={() => {
              setIsNew(true)
              setEditing({ ...emptyConfig })
            }}
          >
            <Plus className="mr-1 h-4 w-4" /> 新增
          </Button>
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>AppID</TableHead>
              <TableHead>商户号</TableHead>
              <TableHead>证书序列号</TableHead>
              <TableHead>回调地址</TableHead>
              <TableHead>审核流</TableHead>
              <TableHead>启用</TableHead>
              <TableHead className="w-24">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {(configs.data?.data || []).map((c) => (
              <TableRow key={c.id}>
                <TableCell>{c.appID}</TableCell>
                <TableCell>{c.mchID}</TableCell>
                <TableCell className="max-w-40 truncate">{c.mchSerialNo}</TableCell>
                <TableCell className="max-w-64 truncate">{c.notifyURL}</TableCell>
                <TableCell>
                  <StatusBadge status={c.refundApprovalRequired ? "PENDING" : "OFF"} />
                </TableCell>
                <TableCell>{c.enable ? "是" : "否"}</TableCell>
                <TableCell>
                  <div className="flex gap-1">
                    <Button
                      size="icon"
                      variant="ghost"
                      onClick={() => {
                        setIsNew(false)
                        setEditing({ ...c })
                      }}
                    >
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button
                      size="icon"
                      variant="ghost"
                      onClick={() => c.id && deleteConfig.mutate(c.id)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </section>

      {/* 订单查询 */}
      <section className="space-y-2">
        <div className="flex items-center justify-between">
          <h2 className="font-medium">支付订单</h2>
          <div className="flex items-center gap-2">
            <select
              className="rounded border px-2 py-1 text-sm"
              value={orderStatus}
              onChange={(e) => {
                setOrderStatus(e.target.value)
                setOrderPage(1)
                setSelectedOrders(new Set())
              }}
            >
              <option value="">全部</option>
              <option value="CREATED">已下单</option>
              <option value="PAID">已支付</option>
              <option value="CLOSED">已关闭</option>
            </select>
            <Button
              size="sm"
              variant="outline"
              disabled={selectedOrders.size === 0 || batchClose.isPending}
              onClick={() => batchClose.mutate([...selectedOrders])}
            >
              <XCircle className="mr-1 h-4 w-4" /> 批量关单({selectedOrders.size})
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() =>
                downloadBlob(
                  `${ORDER}/export${orderStatus ? `?status=${orderStatus}` : ""}`,
                  "pay_orders.csv",
                )
              }
            >
              <Download className="mr-1 h-4 w-4" /> 导出 CSV
            </Button>
          </div>
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10" />
              <TableHead>订单号</TableHead>
              <TableHead>描述</TableHead>
              <TableHead>金额(分)</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>微信状态</TableHead>
              <TableHead>创建时间</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {(orders.data?.data || []).map((o) => (
              <TableRow key={o.id}>
                <TableCell>
                  <input
                    type="checkbox"
                    checked={selectedOrders.has(o.outTradeNo)}
                    onChange={() => toggleOrder(o.outTradeNo)}
                  />
                </TableCell>
                <TableCell className="font-mono text-xs">{o.outTradeNo}</TableCell>
                <TableCell>{o.description}</TableCell>
                <TableCell>{o.amount}</TableCell>
                <TableCell>
                  <StatusBadge status={o.status} />
                </TableCell>
                <TableCell>{o.tradeState}</TableCell>
                <TableCell>{o.createdAt}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        {/* 分页条 */}
        <div className="flex items-center justify-between text-sm text-muted-foreground">
          <span>
            共 {orders.data?.total ?? 0} 条 · 第 {orderPage} /{" "}
            {Math.max(1, orders.data?.pages ?? 1)} 页
          </span>
          <div className="flex gap-2">
            <Button
              size="sm"
              variant="outline"
              disabled={orderPage <= 1}
              onClick={() => {
                setOrderPage(orderPage - 1)
                setSelectedOrders(new Set())
              }}
            >
              上一页
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={orderPage >= (orders.data?.pages ?? 1)}
              onClick={() => {
                setOrderPage(orderPage + 1)
                setSelectedOrders(new Set())
              }}
            >
              下一页
            </Button>
          </div>
        </div>
        {/* 批量关单失败明细 */}
        {closeFailures.length > 0 && (
          <div className="rounded border border-red-300 bg-red-500/5 p-2">
            <div className="mb-1 flex items-center justify-between">
              <span className="text-sm font-medium text-red-600">
                关单失败 {closeFailures.length} 笔
              </span>
              <Button size="sm" variant="ghost" onClick={() => setCloseFailures([])}>
                关闭
              </Button>
            </div>
            <div className="max-h-40 overflow-y-auto">
              {closeFailures.map((f) => (
                <div key={f.outTradeNo} className="font-mono text-xs">
                  {f.outTradeNo} — {f.error}
                </div>
              ))}
            </div>
          </div>
        )}
      </section>

      {/* 退款单(含审核记录) */}
      <section className="space-y-2">
        <div className="flex items-center justify-between">
          <h2 className="font-medium">退款单</h2>
          <select
            className="rounded border px-2 py-1 text-sm"
            value={refundStatus}
            onChange={(e) => setRefundStatus(e.target.value)}
          >
            <option value="PENDING">待审核</option>
            <option value="">全部</option>
            <option value="PROCESSING">受理中</option>
            <option value="SUCCESS">已成功</option>
            <option value="REJECTED">已拒绝</option>
            <option value="CLOSED">已关闭</option>
            <option value="ABNORMAL">异常</option>
          </select>
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>退款单号</TableHead>
              <TableHead>订单号</TableHead>
              <TableHead>金额(分)</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>原因类别</TableHead>
              <TableHead>说明</TableHead>
              <TableHead>审批人 / 意见 / 时间</TableHead>
              <TableHead className="w-40">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {(pendingRefunds.data?.data || []).map((r) => (
              <TableRow key={r.id}>
                <TableCell className="font-mono text-xs">{r.outRefundNo}</TableCell>
                <TableCell className="font-mono text-xs">{r.outTradeNo}</TableCell>
                <TableCell>{r.amount}</TableCell>
                <TableCell>
                  <StatusBadge status={r.status} />
                </TableCell>
                <TableCell>{r.reasonCode}</TableCell>
                <TableCell>{r.reason}</TableCell>
                <TableCell className="max-w-56 text-xs">
                  {r.approverID ? (
                    <div className="space-y-0.5">
                      <div>{r.approverID}</div>
                      {r.approveComment && (
                        <div className="text-muted-foreground">{r.approveComment}</div>
                      )}
                      {r.approvedAt && (
                        <div className="text-muted-foreground">
                          {new Date(r.approvedAt).toLocaleString()}
                        </div>
                      )}
                    </div>
                  ) : (
                    <span className="text-muted-foreground">—</span>
                  )}
                </TableCell>
                <TableCell>
                  {r.status === "PENDING" ? (
                    <div className="flex gap-1">
                      <Button size="sm" onClick={() => openApprove(r, true)}>
                        通过
                      </Button>
                      <Button size="sm" variant="destructive" onClick={() => openApprove(r, false)}>
                        拒绝
                      </Button>
                    </div>
                  ) : null}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </section>

      {/* 退款原因趋势 */}
      <section className="space-y-2">
        <h2 className="font-medium">退款原因趋势(近6个月)</h2>
        {(trend.data || []).length === 0 ? (
          <div className="py-6 text-center text-sm text-muted-foreground">暂无数据</div>
        ) : (
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart
                data={(trend.data || []).map((p) => ({
                  ...p,
                  label: `${p.month} ${p.reasonCode}`,
                }))}
                margin={{ left: 8, right: 16 }}
              >
                <XAxis dataKey="label" tick={{ fontSize: 11 }} interval={0} angle={-20} textAnchor="end" height={60} />
                <YAxis tick={{ fontSize: 11 }} />
                <ChartTooltip formatter={(v: unknown) => Number(v).toLocaleString() + " 分"} />
                <Bar dataKey="amount" fill="#f59e0b" />
              </BarChart>
            </ResponsiveContainer>
          </div>
        )}
      </section>

      {/* 账单归档 */}
      <section className="space-y-2">
        <div className="flex items-center justify-between">
          <h2 className="font-medium">已归档交易账单</h2>
          <div className="flex items-center gap-2">
            <select
              className="rounded border px-2 py-1 text-sm"
              value={billDays}
              onChange={(e) => {
                setBillDays(Number(e.target.value))
                setBillPage(1)
              }}
            >
              <option value={7}>近 7 天</option>
              <option value={30}>近 30 天</option>
              <option value={90}>近 90 天</option>
              <option value={365}>近 365 天</option>
            </select>
            <Input
              placeholder="按商户配置ID过滤(可空)"
              className="w-64"
              value={billConfigID}
              onChange={(e) => {
                setBillConfigID(e.target.value)
                setBillPage(1)
              }}
            />
          </div>
        </div>
        <div className="flex flex-wrap items-end gap-2 rounded border bg-muted/30 p-3">
          <div className="space-y-1">
            <Label className="text-xs">商户配置ID</Label>
            <Input
              className="w-56"
              value={billConfigIDPull}
              onChange={(e) => setBillConfigIDPull(e.target.value)}
              placeholder="configID"
            />
          </div>
          <div className="space-y-1">
            <Label className="text-xs">账单日期(昨日及更早)</Label>
            <Input
              type="date"
              className="w-40"
              value={billDate}
              max={fmtLocalDate(new Date(Date.now() - 86400000))}
              onChange={(e) => setBillDate(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label className="text-xs">类型</Label>
            <select
              className="rounded border px-2 py-2 text-sm"
              value={billType}
              onChange={(e) => setBillType(e.target.value)}
            >
              <option value="ALL">全部流水</option>
              <option value="SUCCESS">仅成功</option>
              <option value="REFUND">仅退款</option>
            </select>
          </div>
          <Button size="sm" disabled={pulling || !billConfigIDPull} onClick={pullTradeBill}>
            <Download className="mr-1 h-4 w-4" />
            {pulling ? "拉取中..." : "手动拉取账单"}
          </Button>
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>账单日期</TableHead>
              <TableHead>类型</TableHead>
              <TableHead>租户ID</TableHead>
              <TableHead className="w-24">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {billRows.map((b) => (
              <TableRow key={b.id}>
                <TableCell>{b.billDate}</TableCell>
                <TableCell>
                  <Badge variant="secondary">{b.billType}</Badge>
                </TableCell>
                <TableCell className="font-mono text-xs">{b.tenantID}</TableCell>
                <TableCell>
                  <Button size="sm" variant="outline" onClick={() => downloadBlob(`${BILL}/download?id=${b.id}`, `trade_bill_${b.billDate}.csv`)}>
                    <Download className="mr-1 h-4 w-4" /> 下载
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        {/* 账单分页条 */}
        <div className="flex items-center justify-between text-sm text-muted-foreground">
          <span>
            共 {bills.data?.total ?? 0} 条 · 第 {billPage} / {billTotalPages} 页
          </span>
          <div className="flex gap-2">
            <Button
              size="sm"
              variant="outline"
              disabled={billPage <= 1}
              onClick={() => setBillPage(billPage - 1)}
            >
              上一页
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={billPage >= billTotalPages}
              onClick={() => setBillPage(billPage + 1)}
            >
              下一页
            </Button>
          </div>
        </div>
      </section>

      {/* 商户配置编辑对话框 */}
      <Dialog open={editing !== null} onOpenChange={(v) => !v && setEditing(null)}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{isNew ? "新增商户配置" : "编辑商户配置"}</DialogTitle>
          </DialogHeader>
          {editing && (
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label>租户ID</Label>
                <Input
                  value={editing.tenantID}
                  onChange={(e) => setEditing({ ...editing, tenantID: e.target.value })}
                />
              </div>
              <div className="space-y-1">
                <Label>微信应用配置ID</Label>
                <Input
                  value={editing.wechatConfigID}
                  onChange={(e) => setEditing({ ...editing, wechatConfigID: e.target.value })}
                />
              </div>
              <div className="space-y-1">
                <Label>AppID</Label>
                <Input
                  value={editing.appID}
                  onChange={(e) => setEditing({ ...editing, appID: e.target.value })}
                />
              </div>
              <div className="space-y-1">
                <Label>商户号</Label>
                <Input
                  value={editing.mchID}
                  onChange={(e) => setEditing({ ...editing, mchID: e.target.value })}
                />
              </div>
              <div className="space-y-1">
                <Label>API证书序列号</Label>
                <Input
                  value={editing.mchSerialNo}
                  onChange={(e) => setEditing({ ...editing, mchSerialNo: e.target.value })}
                />
              </div>
              <div className="space-y-1">
                <Label>APIv3密钥 {isNew ? "" : "(留空保持)"}</Label>
                <Input
                  value={editing.apiV3Key}
                  onChange={(e) => setEditing({ ...editing, apiV3Key: e.target.value })}
                />
              </div>
              <div className="col-span-2 space-y-1">
                <Label>商户私钥PEM {isNew ? "" : "(留空保持)"}</Label>
                <textarea
                  className="h-20 w-full rounded border px-2 py-1 font-mono text-xs"
                  value={editing.privateKey}
                  onChange={(e) => setEditing({ ...editing, privateKey: e.target.value })}
                />
              </div>
              <div className="col-span-2 space-y-1">
                <Label>支付回调URL</Label>
                <Input
                  value={editing.notifyURL}
                  onChange={(e) => setEditing({ ...editing, notifyURL: e.target.value })}
                />
              </div>
              <div className="col-span-2 space-y-1">
                <Label>退款回调URL(可选)</Label>
                <Input
                  value={editing.refundNotifyURL}
                  onChange={(e) => setEditing({ ...editing, refundNotifyURL: e.target.value })}
                />
              </div>
              <div className="flex items-center gap-2">
                <Switch
                  checked={!!editing.refundApprovalRequired}
                  onCheckedChange={(v) => setEditing({ ...editing, refundApprovalRequired: v })}
                />
                <Label>退款需要审核</Label>
              </div>
              <div className="flex items-center gap-2">
                <Switch
                  checked={!!editing.enable}
                  onCheckedChange={(v) => setEditing({ ...editing, enable: v })}
                />
                <Label>启用</Label>
              </div>
              <div className="col-span-2 space-y-1">
                <Label>描述</Label>
                <Input
                  value={editing.description}
                  onChange={(e) => setEditing({ ...editing, description: e.target.value })}
                />
              </div>
            </div>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditing(null)}>
              取消
            </Button>
            <Button
              disabled={saveConfig.isPending}
              onClick={() =>
                editing &&
                (editing.id
                  ? saveConfig.mutate({
                      ...editing,
                      apiV3Key: editing.apiV3Key || "",
                      privateKey: editing.privateKey || "",
                    })
                  : saveConfig.mutate(editing))
              }
            >
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 退款审核确认对话框 */}
      <Dialog open={approving !== null} onOpenChange={(v) => !v && setApproving(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {approving?.approved ? "通过退款" : "拒绝退款"}
            </DialogTitle>
          </DialogHeader>
          {approving && (
            <div className="space-y-3 text-sm">
              <div className="grid grid-cols-2 gap-2">
                <div>
                  <div className="text-muted-foreground">退款单号</div>
                  <div className="font-mono text-xs">{approving.refund.outRefundNo}</div>
                </div>
                <div>
                  <div className="text-muted-foreground">原订单号</div>
                  <div className="font-mono text-xs">{approving.refund.outTradeNo}</div>
                </div>
                <div>
                  <div className="text-muted-foreground">退款金额(分)</div>
                  <div>{approving.refund.amount}</div>
                </div>
                <div>
                  <div className="text-muted-foreground">原因类别</div>
                  <div>{approving.refund.reasonCode || "other"}</div>
                </div>
              </div>
              <div className="space-y-1">
                <Label>审核意见 {approving.approved ? "(可选)" : "(建议填写拒绝理由)"}</Label>
                <Input
                  value={approveComment}
                  onChange={(e) => setApproveComment(e.target.value)}
                  placeholder="将记入审计日志"
                />
              </div>
              {!approving.approved && (
                <p className="text-xs text-muted-foreground">
                  拒绝后退款单置为 REJECTED,资金不会退回用户,可重新发起退款。
                </p>
              )}
            </div>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setApproving(null)}>
              取消
            </Button>
            <Button
              variant={approving?.approved ? "default" : "destructive"}
              disabled={approveRefund.isPending}
              onClick={() =>
                approving &&
                approveRefund.mutate({
                  outRefundNo: approving.refund.outRefundNo,
                  approved: approving.approved,
                  comment: approveComment,
                })
              }
            >
              确认{approving?.approved ? "通过" : "拒绝"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
