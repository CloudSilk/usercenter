import type { ApiResponse } from "@/lib/types"

const TOKEN_KEY = "uc_admin_token"

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY)
}

function redirectLogin(): void {
  clearToken()
  window.location.href = "/login"
}

class ApiError extends Error {
  code: number
  constructor(code: number, message: string) {
    super(message)
    this.code = code
    this.name = "ApiError"
  }
}

async function request<T>(
  method: string,
  url: string,
  body?: unknown,
  params?: Record<string, string | number | boolean | undefined>,
): Promise<T> {
  const token = getToken()
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  }
  if (token) {
    headers["Authorization"] = `Bearer ${token}`
  }

  let fullUrl = url
  if (params) {
    const searchParams = new URLSearchParams()
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined) {
        searchParams.set(key, String(value))
      }
    }
    const qs = searchParams.toString()
    if (qs) {
      fullUrl += `?${qs}`
    }
  }

  const options: RequestInit = { method, headers }
  if (body !== undefined && method !== "GET") {
    options.body = JSON.stringify(body)
  }

  const res = await fetch(fullUrl, options)

  if (res.status === 401) {
    redirectLogin()
    throw new ApiError(401, "Unauthorized")
  }

  const json: ApiResponse<T> = await res.json()

  if (json.code !== 0) {
    throw new ApiError(json.code, json.message || "Request failed")
  }

  // If the response has records, return the full response shape
  if (json.records !== undefined) {
    return json as unknown as T
  }

  return (json.data ?? json) as T
}

export const api = {
  get<T>(url: string, params?: Record<string, string | number | boolean | undefined>) {
    return request<T>("GET", url, undefined, params)
  },
  post<T>(url: string, body?: unknown) {
    return request<T>("POST", url, body)
  },
  put<T>(url: string, body?: unknown) {
    return request<T>("PUT", url, body)
  },
  del<T>(url: string, body?: unknown) {
    return request<T>("DELETE", url, body)
  },
}

// Typed convenience helpers
export function apiGet<T>(
  url: string,
  params?: Record<string, string | number | boolean | undefined>,
): Promise<T> {
  return api.get<T>(url, params)
}

export function apiPost<T>(url: string, body?: unknown): Promise<T> {
  return api.post<T>(url, body)
}

export function apiPut<T>(url: string, body?: unknown): Promise<T> {
  return api.put<T>(url, body)
}

export function apiDel<T>(url: string, body?: unknown): Promise<T> {
  return api.del<T>(url, body)
}

/**
 * EventSource-based SSE with token in query param.
 * Used for live audit log streaming.
 */
export function fetchSSE(
  url: string,
  token: string,
  onMessage: (data: string) => void,
  onError?: (err: Event) => void,
): EventSource {
  const separator = url.includes("?") ? "&" : "?"
  const es = new EventSource(`${url}${separator}token=${encodeURIComponent(token)}`)
  es.onmessage = (e) => onMessage(e.data)
  if (onError) {
    es.onerror = onError
  }
  return es
}

/**
 * ReadableStream-based SSE (for AI gateway streaming).
 * Uses POST with body, reads Response as text/event-stream.
 */
export async function fetchStream(
  url: string,
  token: string,
  body: unknown,
  onChunk: (chunk: string) => void,
): Promise<void> {
  const res = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify(body),
  })

  if (!res.ok) {
    throw new ApiError(res.status, `Stream request failed: ${res.statusText}`)
  }

  const reader = res.body?.getReader()
  if (!reader) {
    throw new Error("Response body is not readable")
  }

  const decoder = new TextDecoder()
  let buffer = ""

  while (true) {
    const { done, value } = await reader.read()
    if (done) break

    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split("\n")
    buffer = lines.pop() || ""

    for (const line of lines) {
      if (line.startsWith("data: ")) {
        const data = line.slice(6)
        if (data === "[DONE]") return
        onChunk(data)
      }
    }
  }
}
