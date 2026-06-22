import { useState } from "react"
import { Plus, Send, Trash2 } from "lucide-react"

import { getToken } from "@/lib/api"
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"

const METHODS = ["GET", "POST", "PUT", "DELETE"] as const
type Method = (typeof METHODS)[number]

interface ParamRow {
  id: number
  key: string
  value: string
}

interface Result {
  status: number
  statusText: string
  body: string
  durationMs: number
}

let nextId = 1

export default function ApiTester() {
  const [method, setMethod] = useState<Method>("GET")
  const [path, setPath] = useState("/api/core/auth/user/query")
  const [params, setParams] = useState<ParamRow[]>([
    { id: nextId++, key: "pageIndex", value: "1" },
    { id: nextId++, key: "pageSize", value: "10" },
  ])
  const [body, setBody] = useState("")
  const [result, setResult] = useState<Result | null>(null)
  const [sending, setSending] = useState(false)

  function updateParam(id: number, patch: Partial<ParamRow>) {
    setParams((rows) => rows.map((r) => (r.id === id ? { ...r, ...patch } : r)))
  }
  function addParam() {
    setParams((rows) => [...rows, { id: nextId++, key: "", value: "" }])
  }
  function removeParam(id: number) {
    setParams((rows) => rows.filter((r) => r.id !== id))
  }

  async function send() {
    if (!path.trim()) return
    setSending(true)
    const started = performance.now()
    try {
      const token = getToken()
      const headers: Record<string, string> = {
        "Content-Type": "application/json",
      }
      if (token) headers["Authorization"] = `Bearer ${token}`

      // Build URL with query params
      let url = path
      const qs = new URLSearchParams()
      for (const p of params) {
        if (p.key.trim()) qs.set(p.key.trim(), p.value)
      }
      const qsStr = qs.toString()
      if (qsStr) url += `${path.includes("?") ? "&" : "?"}${qsStr}`

      const options: RequestInit = { method, headers }
      if (method !== "GET" && body.trim()) options.body = body

      const res = await fetch(url, options)
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
      <h1 className="text-2xl font-bold">API 测试器</h1>

      <Card>
        <CardHeader>
          <CardTitle>请求</CardTitle>
          <CardDescription>使用当前登录 Token（自动注入 Authorization 头）</CardDescription>
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
              <Label className="text-xs">Path</Label>
              <Input
                className="h-8 font-mono text-xs"
                value={path}
                onChange={(e) => setPath(e.target.value)}
              />
            </div>
            <Button onClick={send} disabled={sending}>
              <Send /> {sending ? "发送中…" : "发送"}
            </Button>
          </div>

          <Tabs defaultValue="params">
            <TabsList>
              <TabsTrigger value="params">Query Params</TabsTrigger>
              <TabsTrigger value="body">Body</TabsTrigger>
            </TabsList>
            <TabsContent value="params" className="space-y-2 pt-3">
              {params.map((p) => (
                <div key={p.id} className="flex items-center gap-2">
                  <Input
                    className="h-8 flex-1 font-mono text-xs"
                    placeholder="key"
                    value={p.key}
                    onChange={(e) => updateParam(p.id, { key: e.target.value })}
                  />
                  <span className="text-muted-foreground">=</span>
                  <Input
                    className="h-8 flex-1 font-mono text-xs"
                    placeholder="value"
                    value={p.value}
                    onChange={(e) => updateParam(p.id, { value: e.target.value })}
                  />
                  <Button
                    size="icon"
                    variant="ghost"
                    title="删除"
                    onClick={() => removeParam(p.id)}
                  >
                    <Trash2 className="text-destructive" />
                  </Button>
                </div>
              ))}
              <Button size="sm" variant="outline" onClick={addParam}>
                <Plus /> 新增参数
              </Button>
            </TabsContent>
            <TabsContent value="body" className="space-y-2 pt-3">
              <Textarea
                rows={10}
                className="font-mono text-xs"
                placeholder='{"key":"value"}'
                value={body}
                onChange={(e) => setBody(e.target.value)}
                disabled={method === "GET"}
              />
              {method === "GET" && (
                <div className="text-xs text-muted-foreground">GET 请求不会发送 body</div>
              )}
            </TabsContent>
          </Tabs>
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
                {result.status === 0
                  ? result.statusText
                  : `${result.status} ${result.statusText}`}
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
