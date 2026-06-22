import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import {
  ChevronRight,
  KeyRound,
  Pencil,
  Plus,
  Power,
  RefreshCw,
  Trash2,
  Zap,
} from "lucide-react"

import { api } from "@/lib/api"
import type { AIKey, AIProvider, ModelRoute } from "@/lib/types"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Textarea } from "@/components/ui/textarea"
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

const AUTH_TYPES = ["bearer", "header", "query"]

interface ProviderForm {
  id: string
  name: string
  baseURL: string
  authType: string
  description: string
}
const emptyProvider: ProviderForm = {
  id: "",
  name: "",
  baseURL: "",
  authType: "bearer",
  description: "",
}

interface KeyForm {
  id: string
  name: string
  apiKey: string
  priority: number
  enable: boolean
}
const emptyKey: KeyForm = {
  id: "",
  name: "",
  apiKey: "",
  priority: 0,
  enable: true,
}

interface RouteForm {
  id: string
  modelAlias: string
  providerID: string
  priority: number
  enable: boolean
}
const emptyRoute: RouteForm = {
  id: "",
  modelAlias: "",
  providerID: "",
  priority: 0,
  enable: true,
}

export default function AIKeys() {
  const qc = useQueryClient()

  // ---- Providers ----
  const { data: providers = [], isLoading: pLoading } = useQuery<AIProvider[]>({
    queryKey: ["ai-providers"],
    queryFn: () => api.get<AIProvider[]>("/admin/api/ai-providers"),
  })

  const [pForm, setPForm] = useState<ProviderForm>(emptyProvider)
  const [pOpen, setPOpen] = useState(false)

  const pAddMut = useMutation({
    mutationFn: (b: Omit<ProviderForm, "id">) =>
      api.post("/admin/api/ai-providers", b),
    onSuccess: () => {
      toast.success("供应商已创建")
      setPOpen(false)
      qc.invalidateQueries({ queryKey: ["ai-providers"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const pUpdMut = useMutation({
    mutationFn: (b: ProviderForm) =>
      api.put(`/admin/api/ai-providers/${b.id}`, {
        name: b.name,
        baseURL: b.baseURL,
        authType: b.authType,
        description: b.description,
      }),
    onSuccess: () => {
      toast.success("供应商已更新")
      setPOpen(false)
      qc.invalidateQueries({ queryKey: ["ai-providers"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const pDelMut = useMutation({
    mutationFn: (id: string) => api.del(`/admin/api/ai-providers/${id}`),
    onSuccess: () => {
      toast.success("供应商已删除")
      qc.invalidateQueries({ queryKey: ["ai-providers"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  // ---- Keys drawer ----
  const [keyProvider, setKeyProvider] = useState<AIProvider | null>(null)
  const { data: keys = [], isLoading: kLoading } = useQuery<AIKey[]>({
    queryKey: ["ai-keys", keyProvider?.id],
    queryFn: () =>
      api.get<AIKey[]>("/admin/api/ai-keys", {
        providerID: keyProvider?.id ?? "",
      }),
    enabled: !!keyProvider,
  })

  const [kForm, setKForm] = useState<KeyForm>(emptyKey)
  const [kOpen, setKOpen] = useState(false)

  const kAddMut = useMutation({
    mutationFn: (b: Omit<KeyForm, "id">) =>
      api.post("/admin/api/ai-keys", {
        providerID: keyProvider?.id,
        name: b.name,
        apiKey: b.apiKey,
        priority: b.priority,
        enable: b.enable,
      }),
    onSuccess: () => {
      toast.success("密钥已添加")
      setKOpen(false)
      qc.invalidateQueries({ queryKey: ["ai-keys", keyProvider?.id] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const kUpdMut = useMutation({
    mutationFn: (b: KeyForm) =>
      api.put(`/admin/api/ai-keys/${b.id}`, {
        providerID: keyProvider?.id,
        name: b.name,
        priority: b.priority,
        enable: b.enable,
      }),
    onSuccess: () => {
      toast.success("密钥已更新")
      setKOpen(false)
      qc.invalidateQueries({ queryKey: ["ai-keys", keyProvider?.id] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const kDelMut = useMutation({
    mutationFn: (id: string) => api.del(`/admin/api/ai-keys/${id}`),
    onSuccess: () => {
      toast.success("密钥已删除")
      qc.invalidateQueries({ queryKey: ["ai-keys", keyProvider?.id] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const kToggleMut = useMutation({
    mutationFn: (vars: { k: AIKey; enable: boolean }) =>
      api.put(`/admin/api/ai-keys/${vars.k.id}`, {
        providerID: vars.k.providerID,
        name: vars.k.name,
        priority: vars.k.priority,
        enable: vars.enable,
      }),
    onSuccess: (_d, vars) => {
      toast.success(vars.enable ? "已启用" : "已禁用")
      qc.invalidateQueries({ queryKey: ["ai-keys", keyProvider?.id] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const [rotateTarget, setRotateTarget] = useState<AIKey | null>(null)
  const [rotateKey, setRotateKey] = useState("")
  const rotateMut = useMutation({
    mutationFn: (vars: { id: string; newAPIKey: string }) =>
      api.post(`/admin/api/ai-keys/${vars.id}/rotate`, { newAPIKey: vars.newAPIKey }),
    onSuccess: () => {
      toast.success("密钥已轮转")
      setRotateTarget(null)
      setRotateKey("")
      qc.invalidateQueries({ queryKey: ["ai-keys", keyProvider?.id] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const [testing, setTesting] = useState<string | null>(null)
  async function testKey(k: AIKey) {
    setTesting(k.id)
    try {
      const res = await api.post<{
        success: boolean
        statusCode: number
        latencyMs: number
        model: string
        error: string
        snippet: string
      }>(`/admin/api/ai-keys/${k.id}/test?model=gpt-3.5-turbo`)
      if (res.success) {
        toast.success(
          `测试成功 · ${res.model} · ${res.latencyMs}ms${
            res.snippet ? " · " + res.snippet.slice(0, 80) : ""
          }`,
        )
      } else {
        toast.error(
          `测试失败 · HTTP ${res.statusCode}${res.error ? " · " + res.error : ""}`,
        )
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "测试失败")
    } finally {
      setTesting(null)
    }
  }

  // ---- Model routes ----
  const { data: routes = [], isLoading: rLoading } = useQuery<ModelRoute[]>({
    queryKey: ["model-routes"],
    queryFn: () => api.get<ModelRoute[]>("/admin/api/model-routes"),
  })

  const [rForm, setRForm] = useState<RouteForm>(emptyRoute)
  const [rOpen, setROpen] = useState(false)

  const rAddMut = useMutation({
    mutationFn: (b: Omit<RouteForm, "id">) =>
      api.post("/admin/api/model-routes", b),
    onSuccess: () => {
      toast.success("路由已添加")
      setROpen(false)
      qc.invalidateQueries({ queryKey: ["model-routes"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const rDelMut = useMutation({
    mutationFn: (id: string) => api.del(`/admin/api/model-routes/${id}`),
    onSuccess: () => {
      toast.success("路由已删除")
      qc.invalidateQueries({ queryKey: ["model-routes"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  function providerName(id: string): string {
    return providers.find((p) => p.id === id)?.name || id || "-"
  }

  return (
    <div className="space-y-6 p-4 md:p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-bold">AI 供应商与密钥</h1>
        <Button
          size="sm"
          onClick={() => {
            setPForm({ ...emptyProvider })
            setPOpen(true)
          }}
        >
          <Plus /> 新增供应商
        </Button>
      </div>

      {/* Providers */}
      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>名称</TableHead>
              <TableHead>BaseURL</TableHead>
              <TableHead>鉴权</TableHead>
              <TableHead>健康</TableHead>
              <TableHead>说明</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {pLoading ? (
              <TableRow>
                <TableCell colSpan={6} className="py-8 text-center text-muted-foreground">
                  加载中…
                </TableCell>
              </TableRow>
            ) : providers.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="py-8 text-center text-muted-foreground">
                  暂无数据
                </TableCell>
              </TableRow>
            ) : (
              providers.map((p) => (
                <TableRow key={p.id}>
                  <TableCell className="font-medium">{p.name}</TableCell>
                  <TableCell className="max-w-64 truncate" title={p.baseURL}>
                    {p.baseURL || "-"}
                  </TableCell>
                  <TableCell>
                    <Badge variant="secondary">{p.authType || "-"}</Badge>
                  </TableCell>
                  <TableCell>
                    {p.healthy ? (
                      <Badge>健康</Badge>
                    ) : (
                      <Badge variant="destructive">异常</Badge>
                    )}
                  </TableCell>
                  <TableCell className="max-w-56 truncate text-muted-foreground" title={p.description}>
                    {p.description || "-"}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="inline-flex gap-1">
                      <Button
                        size="icon"
                        variant="ghost"
                        title="密钥管理"
                        onClick={() => setKeyProvider(p)}
                      >
                        <KeyRound />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        title="编辑"
                        onClick={() => {
                          setPForm({
                            id: p.id,
                            name: p.name ?? "",
                            baseURL: p.baseURL ?? "",
                            authType: p.authType || "bearer",
                            description: p.description ?? "",
                          })
                          setPOpen(true)
                        }}
                      >
                        <Pencil />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        title="删除"
                        onClick={() => {
                          if (window.confirm(`确认删除供应商 ${p.name}？`))
                            pDelMut.mutate(p.id)
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
      </div>

      {/* Model Routes */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold">模型路由</h2>
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              setRForm({ ...emptyRoute })
              setROpen(true)
            }}
          >
            <Plus /> 新增路由
          </Button>
        </div>
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>模型别名</TableHead>
                <TableHead>供应商</TableHead>
                <TableHead>优先级</TableHead>
                <TableHead>启用</TableHead>
                <TableHead className="text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rLoading ? (
                <TableRow>
                  <TableCell colSpan={5} className="py-8 text-center text-muted-foreground">
                    加载中…
                  </TableCell>
                </TableRow>
              ) : routes.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="py-8 text-center text-muted-foreground">
                    暂无数据
                  </TableCell>
                </TableRow>
              ) : (
                routes.map((r) => (
                  <TableRow key={r.id}>
                    <TableCell className="font-medium">{r.modelAlias}</TableCell>
                    <TableCell>{providerName(r.providerID)}</TableCell>
                    <TableCell>{r.priority ?? 0}</TableCell>
                    <TableCell>
                      {r.enable ? <Badge>启用</Badge> : <Badge variant="secondary">禁用</Badge>}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        size="icon"
                        variant="ghost"
                        title="删除"
                        onClick={() => {
                          if (window.confirm(`确认删除路由 ${r.modelAlias}？`))
                            rDelMut.mutate(r.id)
                        }}
                      >
                        <Trash2 className="text-destructive" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
        </Table>
        </div>
      </div>

      {/* Provider dialog */}
      <Dialog open={pOpen} onOpenChange={setPOpen}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>{pForm.id ? "编辑供应商" : "新增供应商"}</DialogTitle>
          </DialogHeader>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1">
              <Label>名称</Label>
              <Input
                value={pForm.name}
                onChange={(e) => setPForm({ ...pForm, name: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>鉴权方式</Label>
              <Select
                value={pForm.authType}
                onValueChange={(v) => setPForm({ ...pForm, authType: v })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {AUTH_TYPES.map((t) => (
                    <SelectItem key={t} value={t}>
                      {t}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="col-span-2 space-y-1">
              <Label>BaseURL</Label>
              <Input
                value={pForm.baseURL}
                placeholder="https://api.openai.com/v1"
                onChange={(e) => setPForm({ ...pForm, baseURL: e.target.value })}
              />
            </div>
            <div className="col-span-2 space-y-1">
              <Label>说明</Label>
              <Textarea
                rows={2}
                value={pForm.description}
                onChange={(e) => setPForm({ ...pForm, description: e.target.value })}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPOpen(false)}>
              取消
            </Button>
            <Button
              onClick={() => {
                if (!pForm.name.trim()) {
                  toast.error("请填写名称")
                  return
                }
                if (pForm.id) pUpdMut.mutate(pForm)
                else {
                  const { id: _id, ...body } = pForm
                  void _id
                  pAddMut.mutate(body)
                }
              }}
              disabled={pAddMut.isPending || pUpdMut.isPending}
            >
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Keys drawer */}
      <Sheet open={!!keyProvider} onOpenChange={(o) => !o && setKeyProvider(null)}>
        <SheetContent className="flex w-full flex-col gap-3 sm:max-w-2xl">
          <SheetHeader>
            <SheetTitle className="flex items-center gap-2">
              <ChevronRight className="size-4" />
              密钥管理 - {keyProvider?.name}
            </SheetTitle>
          </SheetHeader>
          <div className="flex items-center justify-between">
            <span className="text-sm text-muted-foreground">
              共 {keys.length} 个密钥
            </span>
            <Button
              size="sm"
              onClick={() => {
                setKForm({ ...emptyKey })
                setKOpen(true)
              }}
            >
              <Plus /> 新增密钥
            </Button>
          </div>
          <div className="overflow-auto rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>名称</TableHead>
                  <TableHead>Key Hint</TableHead>
                  <TableHead>优先级</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {kLoading ? (
                  <TableRow>
                    <TableCell colSpan={5} className="py-8 text-center text-muted-foreground">
                      加载中…
                    </TableCell>
                  </TableRow>
                ) : keys.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={5} className="py-8 text-center text-muted-foreground">
                      暂无密钥
                    </TableCell>
                  </TableRow>
                ) : (
                  keys.map((k) => {
                    const inCooldown =
                      !!k.cooldownEnd && k.cooldownEnd * 1000 > Date.now()
                    return (
                      <TableRow key={k.id}>
                        <TableCell className="font-medium">{k.name}</TableCell>
                        <TableCell className="font-mono text-xs text-muted-foreground">
                          {k.keyHint || "-"}
                        </TableCell>
                        <TableCell>{k.priority ?? 0}</TableCell>
                        <TableCell>
                          <div className="flex flex-col gap-1">
                            {k.enable ? (
                              <Badge>启用</Badge>
                            ) : (
                              <Badge variant="secondary">禁用</Badge>
                            )}
                            {inCooldown && (
                              <Badge variant="destructive">冷却中</Badge>
                            )}
                          </div>
                        </TableCell>
                        <TableCell className="text-right">
                          <div className="inline-flex gap-1">
                            <Button
                              size="icon"
                              variant="ghost"
                              title="实测"
                              disabled={testing === k.id}
                              onClick={() => testKey(k)}
                            >
                              <Zap className={testing === k.id ? "animate-pulse" : ""} />
                            </Button>
                            <Button
                              size="icon"
                              variant="ghost"
                              title="轮转"
                              onClick={() => {
                                setRotateTarget(k)
                                setRotateKey("")
                              }}
                            >
                              <RefreshCw />
                            </Button>
                            <Button
                              size="icon"
                              variant="ghost"
                              title={k.enable ? "禁用" : "启用"}
                              onClick={() =>
                                kToggleMut.mutate({ k, enable: !k.enable })
                              }
                            >
                              <Power />
                            </Button>
                            <Button
                              size="icon"
                              variant="ghost"
                              title="编辑"
                              onClick={() => {
                                setKForm({
                                  id: k.id,
                                  name: k.name ?? "",
                                  apiKey: "",
                                  priority: k.priority ?? 0,
                                  enable: !!k.enable,
                                })
                                setKOpen(true)
                              }}
                            >
                              <Pencil />
                            </Button>
                            <Button
                              size="icon"
                              variant="ghost"
                              title="删除"
                              onClick={() => {
                                if (window.confirm(`确认删除密钥 ${k.name}？`))
                                  kDelMut.mutate(k.id)
                              }}
                            >
                              <Trash2 className="text-destructive" />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    )
                  })
                )}
              </TableBody>
            </Table>
          </div>

          {/* Key dialog */}
          <Dialog open={kOpen} onOpenChange={setKOpen}>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>{kForm.id ? "编辑密钥" : "新增密钥"}</DialogTitle>
              </DialogHeader>
              <div className="space-y-3">
                <div className="space-y-1">
                  <Label>名称</Label>
                  <Input
                    value={kForm.name}
                    onChange={(e) => setKForm({ ...kForm, name: e.target.value })}
                  />
                </div>
                {!kForm.id && (
                  <div className="space-y-1">
                    <Label>API Key (明文，仅创建时提交)</Label>
                    <Input
                      type="password"
                      value={kForm.apiKey}
                      placeholder="sk-..."
                      onChange={(e) => setKForm({ ...kForm, apiKey: e.target.value })}
                    />
                  </div>
                )}
                <div className="grid grid-cols-2 gap-3">
                  <div className="space-y-1">
                    <Label>优先级</Label>
                    <Input
                      type="number"
                      value={kForm.priority}
                      onChange={(e) =>
                        setKForm({ ...kForm, priority: Number(e.target.value) || 0 })
                      }
                    />
                  </div>
                  <div className="flex items-end gap-2 pb-1">
                    <Switch
                      checked={kForm.enable}
                      onCheckedChange={(v) => setKForm({ ...kForm, enable: v })}
                    />
                    <Label>启用</Label>
                  </div>
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setKOpen(false)}>
                  取消
                </Button>
                <Button
                  onClick={() => {
                    if (!kForm.name.trim()) {
                      toast.error("请填写名称")
                      return
                    }
                    if (!kForm.id && !kForm.apiKey.trim()) {
                      toast.error("请填写 API Key")
                      return
                    }
                    if (kForm.id) kUpdMut.mutate(kForm)
                    else {
                      const { id: _id, ...body } = kForm
                      void _id
                      kAddMut.mutate(body)
                    }
                  }}
                  disabled={kAddMut.isPending || kUpdMut.isPending}
                >
                  保存
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>

          {/* Rotate dialog */}
          <Dialog
            open={!!rotateTarget}
            onOpenChange={(o) => {
              if (!o) {
                setRotateTarget(null)
                setRotateKey("")
              }
            }}
          >
            <DialogContent>
              <DialogHeader>
                <DialogTitle>轮转密钥 - {rotateTarget?.name}</DialogTitle>
              </DialogHeader>
              <div className="space-y-1">
                <Label>新 API Key</Label>
                <Input
                  type="password"
                  value={rotateKey}
                  placeholder="sk-..."
                  onChange={(e) => setRotateKey(e.target.value)}
                />
              </div>
              <DialogFooter>
                <Button
                  variant="outline"
                  onClick={() => {
                    setRotateTarget(null)
                    setRotateKey("")
                  }}
                >
                  取消
                </Button>
                <Button
                  onClick={() => {
                    if (rotateTarget && rotateKey.trim())
                      rotateMut.mutate({ id: rotateTarget.id, newAPIKey: rotateKey })
                  }}
                  disabled={rotateMut.isPending}
                >
                  确认轮转
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </SheetContent>
      </Sheet>

      {/* Route dialog */}
      <Dialog open={rOpen} onOpenChange={setROpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>新增路由</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1">
              <Label>模型别名</Label>
              <Input
                value={rForm.modelAlias}
                placeholder="gpt-4"
                onChange={(e) => setRForm({ ...rForm, modelAlias: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>供应商</Label>
              <Select
                value={rForm.providerID}
                onValueChange={(v) => setRForm({ ...rForm, providerID: v })}
              >
                <SelectTrigger>
                  <SelectValue placeholder="选择供应商" />
                </SelectTrigger>
                <SelectContent>
                  {providers.map((p) => (
                    <SelectItem key={p.id} value={p.id}>
                      {p.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label>优先级</Label>
                <Input
                  type="number"
                  value={rForm.priority}
                  onChange={(e) =>
                    setRForm({ ...rForm, priority: Number(e.target.value) || 0 })
                  }
                />
              </div>
              <div className="flex items-end gap-2 pb-1">
                <Switch
                  checked={rForm.enable}
                  onCheckedChange={(v) => setRForm({ ...rForm, enable: v })}
                />
                <Label>启用</Label>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setROpen(false)}>
              取消
            </Button>
            <Button
              onClick={() => {
                if (!rForm.modelAlias.trim()) {
                  toast.error("请填写模型别名")
                  return
                }
                if (!rForm.providerID) {
                  toast.error("请选择供应商")
                  return
                }
                const { id: _id, ...body } = rForm
                void _id
                rAddMut.mutate(body)
              }}
              disabled={rAddMut.isPending}
            >
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
