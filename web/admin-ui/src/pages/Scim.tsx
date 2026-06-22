import { useState } from "react"
import { Send } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

const METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE"] as const
type Method = (typeof METHODS)[number]

const SCIM_ENDPOINTS = [
  { label: "Users", path: "/scim/v2/Users" },
  { label: "Groups", path: "/scim/v2/Groups" },
  { label: "Me", path: "/scim/v2/Me" },
  { label: "Bulk", path: "/scim/v2/Bulk" },
  { label: "ServiceProviderConfig", path: "/scim/v2/ServiceProviderConfig" },
  { label: "ResourceTypes", path: "/scim/v2/ResourceTypes" },
]

interface Result {
  status: number
  statusText: string
  body: string
  durationMs: number
}

export default function Scim() {
  const [method, setMethod] = useState<Method>("GET")
  const [path, setPath] = useState("/scim/v2/Users")
  const [scimToken, setScimToken] = useState("")
  const [tenantID, setTenantID] = useState("")
  const [body, setBody] = useState("")
  const [result, setResult] = useState<Result | null>(null)
  const [sending, setSending] = useState(false)

  async function send() {
    if (!path.trim()) return
    setSending(true)
    const started = performance.now()
    try {
      const headers: Record<string, string> = {
        "Content-Type": "application/scim+json",
      }
      if (scimToken) headers["Authorization"] = `Bearer ${scimToken}`
      if (tenantID) headers["X-Tenant-ID"] = tenantID

      const options: RequestInit = { method, headers }
      if (method !== "GET" && body.trim()) {
        options.body = body
      }

      const res = await fetch(path, options)
      const text = await res.text()
      let pretty = text
      try {
        pretty = JSON.stringify(JSON.parse(text), null, 2)
      } catch {
        /* keep raw text */
      }
      setResult({
        status: res.status,
        statusText: res.statusText,
        body: pretty,
        durationMs: Math.round(performance.now() - started),
      })
    } catch (e) {
      setResult({
        status: 0,
        statusText: "Network Error",
        body: e instanceof Error ? e.message : String(e),
        durationMs: Math.round(performance.now() - started),
      })
    } finally {
      setSending(false)
    }
  }

  return (
    <div className="space-y-6 p-4 md:p-6">
      <h1 className="text-2xl font-bold">SCIM 测试器</h1>

      <Card>
        <CardHeader>
          <CardTitle>SCIM 端点</CardTitle>
          <CardDescription>
            标准端点前缀 /scim/v2。请求使用 SCIM Bearer Token（独立于登录 Token）。
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
          {SCIM_ENDPOINTS.map((e) => (
            <button
              key={e.path}
              type="button"
              className="flex items-center justify-between rounded-md border px-3 py-2 text-left text-sm hover:bg-accent"
              onClick={() => {
                setMethod("GET")
                setPath(e.path)
              }}
            >
              <span>{e.label}</span>
              <span className="font-mono text-xs text-muted-foreground">{e.path}</span>
            </button>
          ))}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>请求构造</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex flex-wrap items-end gap-2">
            <div className="space-y-1">
              <Label className="text-xs">Method</Label>
              <Select value={method} onValueChange={(v) => setMethod(v as Method)}>
                <SelectTrigger className="h-8 w-32">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {METHODS.map((m) => (
                    <SelectItem key={m} value={m}>
                      {m}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex-1 space-y-1">
              <Label className="text-xs">Path（含查询参数）</Label>
              <Input
                className="h-8 font-mono text-xs"
                placeholder="/scim/v2/Users?count=10"
                value={path}
                onChange={(e) => setPath(e.target.value)}
              />
            </div>
          </div>

          <div className="grid gap-2 sm:grid-cols-2">
            <div className="space-y-1">
              <Label className="text-xs">SCIM Bearer Token</Label>
              <Input
                className="h-8 font-mono text-xs"
                type="password"
                placeholder="SCIM 专用 token"
                value={scimToken}
                onChange={(e) => setScimToken(e.target.value)}
              />
            </div>
            <div className="space-y-1">
              <Label className="text-xs">X-Tenant-ID</Label>
              <Input
                className="h-8"
                value={tenantID}
                onChange={(e) => setTenantID(e.target.value)}
              />
            </div>
          </div>

          {method !== "GET" && (
            <div className="space-y-1">
              <Label className="text-xs">Body（SCIM JSON）</Label>
              <Textarea
                rows={8}
                className="font-mono text-xs"
                placeholder='{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],...}'
                value={body}
                onChange={(e) => setBody(e.target.value)}
              />
            </div>
          )}

          <Button onClick={send} disabled={sending}>
            <Send /> {sending ? "发送中…" : "发送"}
          </Button>
        </CardContent>
      </Card>

      {result && (
        <Card>
          <CardHeader>
            <CardTitle>响应</CardTitle>
            <CardDescription>
              <span
                className={
                  result.status >= 200 && result.status < 300
                    ? "text-green-600"
                    : result.status === 0
                      ? "text-destructive"
                      : "text-amber-600"
                }
              >
                {result.status === 0 ? result.statusText : `${result.status} ${result.statusText}`}
              </span>
              <span className="ml-2 text-xs text-muted-foreground">{result.durationMs} ms</span>
            </CardDescription>
          </CardHeader>
          <CardContent>
            <pre className="bg-zinc-900 text-zinc-100 p-3 rounded text-xs overflow-auto max-h-96">
              {result.body || "(empty body)"}
            </pre>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
