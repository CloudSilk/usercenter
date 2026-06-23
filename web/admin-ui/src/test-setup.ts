import { afterEach, beforeEach, vi } from "vitest"

// 每个 test case 前重置 localStorage + fetch mock
beforeEach(() => {
  localStorage.clear()
})

afterEach(() => {
  vi.restoreAllMocks()
})
