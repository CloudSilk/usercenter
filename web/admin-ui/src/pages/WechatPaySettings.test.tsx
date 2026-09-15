import { describe, it, expect, vi, beforeEach } from "vitest"
import "@testing-library/jest-dom/vitest"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"

import WechatPaySettings from "@/pages/WechatPaySettings"
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

const loopStatus = {
  running: true,
  intervalSeconds: 60,
  scanAgeMinutes: 5,
  batchSize: 200,
  alertAgeHours: 24,
  billRetentionDays: 90,
  lastReconcileAt: "2026-09-13T10:00:00+08:00",
  lastReconcileOK: true,
  lastProcessed: 3,
  lastDailyReport: "2026-09-12",
  lastBillDownload: "2026-09-12",
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <WechatPaySettings />
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mockedApi.get.mockImplementation((u: string) => {
    if (u.includes("loop-status")) {
      return Promise.resolve(loopStatus)
    }
    return Promise.resolve([])
  })
})

async function fieldByLabel(labelText: string) {
  // Label 与 Input 同在一个包裹 div,按 label 文本定位关联输入框
  const labelEl = await screen.findByText(labelText)
  const container = labelEl.closest("div") as HTMLElement
  return container.querySelector("input") as HTMLInputElement
}

describe("WechatPaySettings page", () => {
  it("renders status summary and backfills current values", async () => {
    renderPage()
    expect(await screen.findByText(/对账循环 运行中/)).toBeInTheDocument()
    expect(screen.getByText(/处理 3 笔/)).toBeInTheDocument()
    expect(screen.getByText(/日报 2026-09-12/)).toBeInTheDocument()
    // 回填当前运行值
    expect(await fieldByLabel("轮询间隔(秒)")).toHaveValue(60)
    expect(await fieldByLabel("扫描窗口(分钟)")).toHaveValue(5)
    expect(await fieldByLabel("单轮批次上限")).toHaveValue(200)
    expect(await fieldByLabel("滞留告警阈值(小时)")).toHaveValue(24)
    expect(await fieldByLabel("账单保留期(天)")).toHaveValue(90)
  })

  it("saves edited params via PUT", async () => {
    mockedApi.put.mockResolvedValue({ code: 20000, data: loopStatus })
    renderPage()
    const interval = await fieldByLabel("轮询间隔(秒)")
    await waitFor(() => expect(interval).toHaveValue(60))
    await userEvent.clear(interval)
    await userEvent.type(interval, "120")
    await userEvent.click(screen.getByRole("button", { name: /保存/ }))
    await waitFor(() => {
      expect(mockedApi.put).toHaveBeenCalledWith("/api/core/wechat/pay/stats/config", {
        intervalSeconds: 120,
        scanAgeMinutes: 5,
        batchSize: 200,
        alertAgeHours: 24,
        billRetentionDays: 90,
      })
    })
  })

  it("reset calls DELETE and returns to baseline", async () => {
    mockedApi.del.mockResolvedValue({ code: 20000, data: loopStatus })
    renderPage()
    await userEvent.click(await screen.findByRole("button", { name: /恢复 Nacos 基线/ }))
    await waitFor(() => {
      expect(mockedApi.del).toHaveBeenCalledWith("/api/core/wechat/pay/stats/config")
    })
  })

  it("reconcile-now posts and refreshes", async () => {
    mockedApi.post.mockResolvedValue({ code: 20000, data: { processed: 7 } })
    renderPage()
    await userEvent.click(await screen.findByRole("button", { name: /立即对账/ }))
    await waitFor(() => {
      expect(mockedApi.post).toHaveBeenCalledWith("/api/core/wechat/pay/stats/reconcile-now")
    })
  })
})
