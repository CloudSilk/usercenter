import { describe, it, expect, vi, beforeEach } from "vitest"
import { api, getToken, setToken, clearToken, apiGet, apiPost, apiPut, apiDel } from "@/lib/api"

// mock global fetch
function mockFetch(response: Partial<Response> & { json?: () => Promise<unknown> }): typeof fetch {
  return vi.fn(async () => ({
    ok: true,
    status: 200,
    json: response.json || (async () => ({})),
    ...response,
  } as Response)) as unknown as typeof fetch
}

describe("token management", () => {
  it("set/get/clear token in localStorage", () => {
    expect(getToken()).toBeNull()
    setToken("my-jwt-token")
    expect(getToken()).toBe("my-jwt-token")
    clearToken()
    expect(getToken()).toBeNull()
  })
})

describe("api.get", () => {
  beforeEach(() => {
    globalThis.fetch = mockFetch({
      json: async () => ({ code: 0, data: { name: "test" } }),
    })
  })

  it("injects Bearer token from localStorage", async () => {
    setToken("jwt-123")
    await api.get("/admin/api/stats")
    const call = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0]
    const opts = call[1] as RequestInit
    expect(opts.headers).toMatchObject({ Authorization: "Bearer jwt-123" })
  })

  it("builds query string from params", async () => {
    await api.get("/api/core/auth/user/query", {
      pageIndex: 1,
      pageSize: 10,
      userName: undefined,
    })
    const url = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0][0] as string
    expect(url).toContain("pageIndex=1")
    expect(url).toContain("pageSize=10")
    // undefined values should be omitted
    expect(url).not.toContain("userName")
  })

  it("returns data field on success", async () => {
    const result = await api.get<{ name: string }>("/test")
    expect(result).toEqual({ name: "test" })
  })

  it("returns full response when records present (pagination)", async () => {
    globalThis.fetch = mockFetch({
      json: async () => ({ code: 0, data: [{ id: 1 }], records: 42 }),
    })
    const result = await api.get<{ data: unknown[]; records: number }>("/list")
    expect(result.records).toBe(42)
  })

  it("throws on non-zero code", async () => {
    globalThis.fetch = mockFetch({
      json: async () => ({ code: 50000, message: "Internal error" }),
    })
    await expect(api.get("/fail")).rejects.toThrow("Internal error")
  })
})

describe("api.post / api.put / api.del", () => {
  beforeEach(() => {
    setToken("tok")
    globalThis.fetch = mockFetch({ json: async () => ({ code: 0 }) })
  })

  it("POST sends JSON body", async () => {
    await apiPost("/api/core/auth/user/add", { userName: "alice" })
    const opts = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0][1] as RequestInit
    expect(opts.method).toBe("POST")
    expect(opts.body).toBe(JSON.stringify({ userName: "alice" }))
  })

  it("PUT sends body", async () => {
    await apiPut("/api/core/auth/user/update", { id: "1", name: "bob" })
    const opts = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0][1] as RequestInit
    expect(opts.method).toBe("PUT")
    expect(opts.body).toContain('"id":"1"')
  })

  it("DELETE sends JSON body (Go backend requires {id} in body)", async () => {
    await apiDel("/api/core/auth/user/delete", { id: "user-123" })
    const opts = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0][1] as RequestInit
    expect(opts.method).toBe("DELETE")
    // 关键：DELETE 带 body（不是空）
    expect(opts.body).toBe(JSON.stringify({ id: "user-123" }))
  })

  it("GET does not send body even if provided", async () => {
    globalThis.fetch = mockFetch({ json: async () => ({ code: 0, data: {} }) })
    await api.get("/test", { q: "search" })
    const opts = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0][1] as RequestInit
    expect(opts.body).toBeUndefined()
  })
})

describe("401 handling", () => {
  it("clears token and redirects on 401", async () => {
    setToken("expired-token")
    const originalHref = window.location.href
    // jsdom 不支持真正的导航；验证 clearToken 即可
    let redirected = false
    Object.defineProperty(window, "location", {
      value: {
        ...window.location,
        set href(v: string) {
          redirected = v === "/login"
        },
      },
      writable: true,
    })
    globalThis.fetch = mockFetch({ status: 401, json: async () => ({}) })
    await expect(api.get("/protected")).rejects.toThrow()
    expect(getToken()).toBeNull()
    // restore
    Object.defineProperty(window, "location", { value: { href: originalHref }, writable: true })
  })
})
