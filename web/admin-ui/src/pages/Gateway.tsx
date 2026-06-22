import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Pencil, Play, Plus, Trash2 } from "lucide-react"

import { api, fetchStream, getToken } from "@/lib/api"
import type { PromptTemplate } from "@/lib/types"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
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

interface ModelsResp {
  object: string
  data?: { id: string; object: string }[]
}

interface PromptForm {
  id: string
  name: string
  category: string
  modelAlias: string
  content: string
  variables: string
  enable: boolean
}
const emptyPrompt: PromptForm = {
  id: "",
  name: "",
  category: "",
  modelAlias: "",
  content: "",
  variables: "",
  enable: true,
}

export default function Gateway() {
  return (
    <div className="space-y-4 p-4 md:p-6">
      <h1 className="text-2xl font-bold">AI 网关</h1>
      <Tabs defaultValue="tester">
        <TabsList>
          <TabsTrigger value="tester">Chat Tester</TabsTrigger>
          <TabsTrigger value="prompts">Prompt 模板</TabsTrigger>
        </TabsList>
        <TabsContent value="tester">
          <ChatTester />
        </TabsContent>
        <TabsContent value="prompts">
          <PromptTemplates />
        </TabsContent>
      </Tabs>
    </div>
  )
}

function ChatTester() {
  const { data: modelsResp } = useQuery<ModelsResp>({
    queryKey: ["v1-models"],
    queryFn: () => api.get<ModelsResp>("/v1/models"),
  })
  const modelIds = (modelsResp?.data ?? []).map((m) => m.id)

  const [model, setModel] = useState("")
  const [customModel, setCustomModel] = useState("")
  const [systemPrompt, setSystemPrompt] = useState("")
  const [userMsg, setUserMsg] = useState("")
  const [stream, setStream] = useState(true)

  const [response, setResponse] = useState("")
  const [usage, setUsage] = useState<{
    prompt_tokens?: number
    completion_tokens?: number
    total_tokens?: number
  } | null>(null)
  const [loading, setLoading] = useState(false)

  const effectiveModel = customModel.trim() || model

  async function send() {
    if (!effectiveModel.trim()) {
      toast.error("请选择或输入模型")
      return
    }
    if (!userMsg.trim()) {
      toast.error("请输入消息")
      return
    }
    const token = getToken()
    if (!token) {
      toast.error("未登录")
      return
    }

    const messages: { role: string; content: string }[] = []
    if (systemPrompt.trim()) messages.push({ role: "system", content: systemPrompt })
    messages.push({ role: "user", content: userMsg })

    setResponse("")
    setUsage(null)
    setLoading(true)

    try {
      if (stream) {
        let acc = ""
        let lastUsage: typeof usage = null
        await fetchStream(
          "/v1/chat/completions",
          token,
          { model: effectiveModel, messages, stream: true },
          (data) => {
            try {
              const parsed = JSON.parse(data)
              const delta = parsed?.choices?.[0]?.delta?.content
              if (delta) {
                acc += delta
                setResponse(acc)
              }
              if (parsed?.usage) lastUsage = parsed.usage
            } catch {
              // ignore keepalive lines
            }
          },
        )
        if (lastUsage) setUsage(lastUsage)
      } else {
        const res = await fetch("/v1/chat/completions", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({ model: effectiveModel, messages, stream: false }),
        })
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        const json = await res.json()
        const content = json?.choices?.[0]?.message?.content ?? ""
        setResponse(content)
        if (json?.usage) setUsage(json.usage)
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "请求失败")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">请求</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-1">
            <Label>模型</Label>
            <Select value={model} onValueChange={(v) => setModel(v)}>
              <SelectTrigger>
                <SelectValue placeholder="选择模型" />
              </SelectTrigger>
              <SelectContent>
                {modelIds.map((id) => (
                  <SelectItem key={id} value={id}>
                    {id}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Input
              className="mt-2"
              placeholder="或手动输入模型名（优先）"
              value={customModel}
              onChange={(e) => setCustomModel(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label>System Prompt (可选)</Label>
            <Textarea
              rows={3}
              value={systemPrompt}
              onChange={(e) => setSystemPrompt(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label>用户消息</Label>
            <Textarea
              rows={6}
              value={userMsg}
              onChange={(e) => setUserMsg(e.target.value)}
            />
          </div>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Switch checked={stream} onCheckedChange={setStream} />
              <Label>流式 (Stream)</Label>
            </div>
            <Button onClick={send} disabled={loading}>
              <Play className={loading ? "animate-pulse" : ""} />
              {loading ? "请求中…" : "发送"}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0">
          <CardTitle className="text-base">响应</CardTitle>
          {usage && (
            <div className="flex gap-3 text-xs text-muted-foreground">
              <span>prompt: {usage.prompt_tokens ?? 0}</span>
              <span>completion: {usage.completion_tokens ?? 0}</span>
              <span>total: {usage.total_tokens ?? 0}</span>
            </div>
          )}
        </CardHeader>
        <CardContent>
          <pre className="min-h-64 max-h-[28rem] overflow-auto whitespace-pre-wrap rounded-md bg-zinc-950 p-3 font-mono text-sm text-zinc-100">
            {response || (loading ? "…" : "响应将显示在这里")}
          </pre>
        </CardContent>
      </Card>
    </div>
  )
}

function PromptTemplates() {
  const qc = useQueryClient()

  const { data: prompts = [], isLoading } = useQuery<PromptTemplate[]>({
    queryKey: ["prompts"],
    queryFn: () => api.get<PromptTemplate[]>("/admin/api/prompts"),
  })

  const [form, setForm] = useState<PromptForm>(emptyPrompt)
  const [open, setOpen] = useState(false)

  const [renderTarget, setRenderTarget] = useState<PromptTemplate | null>(null)
  const [renderInput, setRenderInput] = useState("")
  const [renderOutput, setRenderOutput] = useState("")
  const [rendering, setRendering] = useState(false)

  const addMut = useMutation({
    mutationFn: (b: Omit<PromptForm, "id">) => api.post("/admin/api/prompts", b),
    onSuccess: () => {
      toast.success("模板已创建")
      setOpen(false)
      qc.invalidateQueries({ queryKey: ["prompts"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const updMut = useMutation({
    mutationFn: (b: PromptForm) => api.put(`/admin/api/prompts/${b.id}`, b),
    onSuccess: () => {
      toast.success("模板已更新")
      setOpen(false)
      qc.invalidateQueries({ queryKey: ["prompts"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const delMut = useMutation({
    mutationFn: (id: string) => api.del(`/admin/api/prompts/${id}`),
    onSuccess: () => {
      toast.success("模板已删除")
      qc.invalidateQueries({ queryKey: ["prompts"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  async function doRender() {
    if (!renderTarget) return
    const vars: Record<string, string> = {}
    for (const line of renderInput.split("\n")) {
      const idx = line.indexOf("=")
      if (idx > 0) {
        const k = line.slice(0, idx).trim()
        const v = line.slice(idx + 1).trim()
        if (k) vars[k] = v
      }
    }
    setRendering(true)
    setRenderOutput("")
    try {
      const res = await api.post<{
        rendered: string
        variables: unknown
        model: string
      }>(`/admin/api/prompts/${renderTarget.id}/render`, { vars })
      setRenderOutput(res.rendered ?? "")
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "渲染失败")
    } finally {
      setRendering(false)
    }
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0">
        <CardTitle className="text-base">Prompt 模板</CardTitle>
        <Button
          size="sm"
          onClick={() => {
            setForm({ ...emptyPrompt })
            setOpen(true)
          }}
        >
          <Plus /> 新增模板
        </Button>
      </CardHeader>
      <CardContent className="px-2">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>名称</TableHead>
              <TableHead>分类</TableHead>
              <TableHead>模型别名</TableHead>
              <TableHead>变量</TableHead>
              <TableHead>内容预览</TableHead>
              <TableHead>启用</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={7} className="py-8 text-center text-muted-foreground">
                  加载中…
                </TableCell>
              </TableRow>
            ) : prompts.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className="py-8 text-center text-muted-foreground">
                  暂无数据
                </TableCell>
              </TableRow>
            ) : (
              prompts.map((p) => (
                <TableRow key={p.id}>
                  <TableCell className="font-medium">{p.name}</TableCell>
                  <TableCell>{p.category || "-"}</TableCell>
                  <TableCell>{p.modelAlias || "-"}</TableCell>
                  <TableCell className="max-w-32 truncate text-muted-foreground" title={p.variables}>
                    {p.variables || "-"}
                  </TableCell>
                  <TableCell className="max-w-56 truncate text-muted-foreground" title={p.content}>
                    {p.content || "-"}
                  </TableCell>
                  <TableCell>
                    {p.enable ? <Badge>启用</Badge> : <Badge variant="secondary">禁用</Badge>}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="inline-flex gap-1">
                      <Button
                        size="icon"
                        variant="ghost"
                        title="渲染测试"
                        onClick={() => {
                          setRenderTarget(p)
                          setRenderInput(
                            (p.variables || "")
                              .split(",")
                              .map((v) => v.trim())
                              .filter(Boolean)
                              .map((v) => `${v}=`)
                              .join("\n"),
                          )
                          setRenderOutput("")
                        }}
                      >
                        <Play />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        title="编辑"
                        onClick={() => {
                          setForm({
                            id: p.id,
                            name: p.name ?? "",
                            category: p.category ?? "",
                            modelAlias: p.modelAlias ?? "",
                            content: p.content ?? "",
                            variables: p.variables ?? "",
                            enable: !!p.enable,
                          })
                          setOpen(true)
                        }}
                      >
                        <Pencil />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        title="删除"
                        onClick={() => {
                          if (window.confirm(`确认删除模板 ${p.name}？`))
                            delMut.mutate(p.id)
                        }}
                      >
                        <Trash2 className="text-destructive" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </CardContent>

      {/* Add / Edit dialog */}
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>{form.id ? "编辑模板" : "新增模板"}</DialogTitle>
          </DialogHeader>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1">
              <Label>名称</Label>
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>分类</Label>
              <Input
                value={form.category}
                onChange={(e) => setForm({ ...form, category: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>模型别名</Label>
              <Input
                value={form.modelAlias}
                placeholder="gpt-4"
                onChange={(e) => setForm({ ...form, modelAlias: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>变量 (逗号分隔)</Label>
              <Input
                value={form.variables}
                placeholder="name,topic"
                onChange={(e) => setForm({ ...form, variables: e.target.value })}
              />
            </div>
            <div className="col-span-2 space-y-1">
              <Label>内容 (支持 {"{{var}}"} 模板变量)</Label>
              <Textarea
                rows={6}
                value={form.content}
                onChange={(e) => setForm({ ...form, content: e.target.value })}
              />
            </div>
            <div className="col-span-2 flex items-center gap-2">
              <Switch
                checked={form.enable}
                onCheckedChange={(v) => setForm({ ...form, enable: v })}
              />
              <Label>启用</Label>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              取消
            </Button>
            <Button
              onClick={() => {
                if (!form.name.trim()) {
                  toast.error("请填写名称")
                  return
                }
                if (form.id) updMut.mutate(form)
                else {
                  const { id: _id, ...body } = form
                  void _id
                  addMut.mutate(body)
                }
              }}
              disabled={addMut.isPending || updMut.isPending}
            >
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Render dialog */}
      <Dialog
        open={!!renderTarget}
        onOpenChange={(o) => {
          if (!o) {
            setRenderTarget(null)
            setRenderInput("")
            setRenderOutput("")
          }
        }}
      >
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>渲染测试 - {renderTarget?.name}</DialogTitle>
          </DialogHeader>
          <div className="space-y-1">
            <Label>变量 (每行 key=value)</Label>
            <Textarea
              rows={5}
              className="font-mono text-sm"
              value={renderInput}
              onChange={(e) => setRenderInput(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label>渲染结果</Label>
            <pre className="min-h-24 rounded-md bg-zinc-950 p-3 font-mono text-sm whitespace-pre-wrap text-zinc-100">
              {rendering ? "渲染中…" : renderOutput || "（点击渲染）"}
            </pre>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setRenderTarget(null)
                setRenderInput("")
                setRenderOutput("")
              }}
            >
              取消
            </Button>
            <Button onClick={doRender} disabled={rendering}>
              渲染
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  )
}
