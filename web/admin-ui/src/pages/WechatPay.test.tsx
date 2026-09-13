import { describe, it, expect, vi, beforeEach } from "vitest"
import "@testing-library/jest-dom/vitest"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"

import WechatPay from "@/pages/WechatPay"
import { api } from "@/lib/api"

vi.mock("@/lib/api", () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    del: vi.fn(),
  },
  getToken: vi.fn(() => "jwt-test"),
}))

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

const mockedApi = vi.mocked(api)

const cfg = {
  id: "cfg-1",
  tenantID: "t1",
  wechatConfigID: "wc-1",
  appID: "wx-app",
  mchID: "1900000000",
  mchSerialNo: "serial-abc",
  notifyURL: "https://h.example.com/notify",
  refundApprovalRequired: false,
  enable: true,
}

const order = {
  id: "o1",
  tenantID: "t1",
  userID: "u1",
  mchID: "1900000000",
  outTradeNo: "order-no-1",
  description: "会员",
  amount: 990,
  status: "PAID",
  tradeState: "SUCCESS",
  createdAt: "2026-09-13T10:00:00+08:00",
}

const pendingRefund = {
  id: "r1",
  outTradeNo: "order-no-1",
  outRefundNo: "rf-test-0001",
  amount: 100,
  reasonCode: "quality",
  reason: "划痕",
  status: "PENDING",
}

function mockGetByUrl(url: string) {
  mockedApi.get.mockImplementation((u: string) => {
    if (u.startsWith("/api/core/wechat/pay/config/query")) {
      return Promise.resolve({ data: [cfg], records: 1 })
    }
    if (u.startsWith("/api/core/wechat/pay/order/query")) {
      return Promise.resolve({ data: [order], records: 1 })
    }
    if (u.startsWith("/api/core/wechat/pay/refund/query")) {
      return Promise.resolve({ data: [pendingRefund], records: 1 })
    }
    if (u.includes("loop-status")) {
      return Promise.resolve({
        running: true,
        intervalSeconds: 60,
        scanAgeMinutes: 5,
        batchSize: 200,
        alertAgeHours: 24,
        billRetentionDays: 90,
      })
    }
    if (u.includes("refund-reason/trend")) {
      return Promise.resolve([])
    }
    if (u.startsWith("/api/core/wechat/pay/bill/list")) {
      return Promise.resolve({ data: [] })
    }
    return Promise.resolve({ data: [] })
  })
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <WechatPay />
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mockGetByUrl("")
})

describe("WechatPay page", () => {
  it("renders merchant configs, orders and pending refunds", async () => {
    renderPage()
    // 商户配置
    expect(await screen.findByText("wx-app")).toBeInTheDocument()
    expect(screen.getByText("1900000000")).toBeInTheDocument()
    // 订单(订单号同时出现在退款表中,用 findAllByText)
    const orderCells = await screen.findAllByText("order-no-1")
    expect(orderCells.length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText("会员")).toBeInTheDocument()
    // 待审核退款
    expect(await screen.findByText("rf-test-0001")).toBeInTheDocument()
    expect(screen.getByText("quality")).toBeInTheDocument()
  })

  it("shows loop status snapshot", async () => {
    renderPage()
    expect(await screen.findByText(/对账循环 运行中/)).toBeInTheDocument()
    expect(screen.getByText(/batchSize|批次/)).toBeTruthy()
  })

  it("approve posts to refund approve endpoint with approved=true", async () => {
    mockedApi.post.mockResolvedValue({ code: 20000, data: {} })
    renderPage()
    const approve = await screen.findByRole("button", { name: "通过" })
    await userEvent.click(approve)
    // 对话框出现,填意见并确认
    const comment = await screen.findByPlaceholderText("将记入审计日志")
    await userEvent.type(comment, "已核实")
    await userEvent.click(screen.getByRole("button", { name: "确认通过" }))
    await waitFor(() => {
      expect(mockedApi.post).toHaveBeenCalledWith("/api/core/wechat/pay/refund/approve", {
        outRefundNo: "rf-test-0001",
        approved: true,
        comment: "已核实",
      })
    })
  })

  it("reject posts approved=false", async () => {
    mockedApi.post.mockResolvedValue({ code: 20000, data: {} })
    renderPage()
    const reject = await screen.findByRole("button", { name: "拒绝" })
    await userEvent.click(reject)
    await userEvent.click(await screen.findByRole("button", { name: "确认拒绝" }))
    await waitFor(() => {
      expect(mockedApi.post).toHaveBeenCalledWith("/api/core/wechat/pay/refund/approve", {
        outRefundNo: "rf-test-0001",
        approved: false,
        comment: "",
      })
    })
  })

  it("batch close posts selected trade numbers", async () => {
    mockedApi.post.mockResolvedValue({ code: 20000, data: { closed: 1, skipped: 0 } })
    renderPage()
    const checkbox = await screen.findByRole("checkbox")
    await userEvent.click(checkbox)
    await userEvent.click(screen.getByRole("button", { name: /批量关单\(1\)/ }))
    await waitFor(() => {
      expect(mockedApi.post).toHaveBeenCalledWith("/api/core/wechat/pay/order/batch-close", {
        outTradeNos: ["order-no-1"],
      })
    })
  })
})
