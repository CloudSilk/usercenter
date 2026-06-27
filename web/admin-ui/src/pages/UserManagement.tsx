import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { KeyRound, Pencil, Plus, Power, Trash2 } from "lucide-react"

import { api } from "@/lib/api"
import type { User } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { Switch } from "@/components/ui/switch"
import { Checkbox } from "@/components/ui/checkbox"
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

interface UserListResponse {
  data: User[]
  records: number
}

interface UserFormState {
  id: string
  userName: string
  nickname: string
  password: string
  mobile: string
  email: string
  enable: boolean
  roleIDs: string[]
}

const emptyForm: UserFormState = {
  id: "",
  userName: "",
  nickname: "",
  password: "",
  mobile: "",
  email: "",
  enable: true,
  roleIDs: [],
}

export default function UserManagement() {
  const qc = useQueryClient()

  const [search, setSearch] = useState({ userName: "", tenantID: "" })
  const [applied, setApplied] = useState({ userName: "", tenantID: "" })
  const [pageIndex, setPageIndex] = useState(1)
  const pageSize = 10

  const [editOpen, setEditOpen] = useState(false)
  const [form, setForm] = useState<UserFormState>(emptyForm)
  const [resetTarget, setResetTarget] = useState<User | null>(null)
  const [resetPwd, setResetPwd] = useState("")

  const listKey = ["user-management", applied, pageIndex, pageSize]

  const { data, isLoading } = useQuery<UserListResponse>({
    queryKey: listKey,
    queryFn: () =>
      api.get<UserListResponse>("/api/core/auth/user/query", {
        pageIndex,
        pageSize,
        userName: applied.userName || undefined,
        tenantID: applied.tenantID || undefined,
      }),
  })

  const { data: roles } = useQuery<any[]>({
    queryKey: ["roles-for-um"],
    queryFn: () => api.get<any[]>("/api/core/auth/role/query"),
  })

  const users = data?.data ?? []
  const total = data?.records ?? 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  function toggleRole(id: string, on: boolean) {
    setForm((f) => ({
      ...f,
      roleIDs: on
        ? [...new Set([...f.roleIDs, id])]
        : f.roleIDs.filter((r) => r !== id),
    }))
  }

  const addMutation = useMutation({
    mutationFn: (body: any) => api.post("/api/core/auth/user/add", body),
    onSuccess: () => {
      toast.success("User created")
      setEditOpen(false)
      qc.invalidateQueries({ queryKey: ["user-management"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const updateMutation = useMutation({
    mutationFn: (body: any) => api.put("/api/core/auth/user/update", body),
    onSuccess: () => {
      toast.success("User updated")
      setEditOpen(false)
      qc.invalidateQueries({ queryKey: ["user-management"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.del("/api/core/auth/user/delete", { id }),
    onSuccess: () => {
      toast.success("User deleted")
      qc.invalidateQueries({ queryKey: ["user-management"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const enableMutation = useMutation({
    mutationFn: (vars: { id: string; enable: boolean }) =>
      api.post("/api/core/auth/user/enable", vars),
    onSuccess: (_d, vars) => {
      toast.success(vars.enable ? "Enabled" : "Disabled")
      qc.invalidateQueries({ queryKey: ["user-management"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const resetMutation = useMutation({
    mutationFn: (vars: { id: string; password: string }) =>
      api.post("/api/core/auth/user/resetpwd", vars),
    onSuccess: () => {
      toast.success("Password reset")
      setResetTarget(null)
      setResetPwd("")
    },
    onError: (e: Error) => toast.error(e.message),
  })

  function openAdd() {
    setForm({ ...emptyForm })
    setEditOpen(true)
  }

  function openEdit(u: User) {
    setForm({
      id: u.id,
      userName: u.userName ?? "",
      nickname: u.nickname ?? "",
      password: "",
      mobile: u.mobile ?? "",
      email: u.email ?? "",
      enable: !!u.enable,
      roleIDs: u.roleIDs ?? [],
    })
    setEditOpen(true)
  }

  function submitForm() {
    if (!form.userName.trim()) {
      toast.error("Please enter username")
      return
    }
    if (form.id) {
      const { password: _pw, ...rest } = form
      void _pw
      updateMutation.mutate(rest)
    } else {
      addMutation.mutate(form)
    }
  }

  return (
    <div className="space-y-4 p-4 md:p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-bold">User Management</h1>
        <Button size="sm" onClick={openAdd}>
          <Plus /> Add User
        </Button>
      </div>

      <div className="flex flex-wrap items-end gap-2">
        <div className="space-y-1">
          <Label className="text-xs">User Name</Label>
          <Input
            className="h-8 w-44"
            value={search.userName}
            onChange={(e) => setSearch({ ...search, userName: e.target.value })}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                setApplied({ ...search })
                setPageIndex(1)
              }
            }}
          />
        </div>
        <div className="space-y-1">
          <Label className="text-xs">Tenant ID</Label>
          <Input
            className="h-8 w-44"
            value={search.tenantID}
            onChange={(e) => setSearch({ ...search, tenantID: e.target.value })}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                setApplied({ ...search })
                setPageIndex(1)
              }
            }}
          />
        </div>
        <Button
          size="sm"
          onClick={() => {
            setApplied({ ...search })
            setPageIndex(1)
          }}
        >
          Search
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={() => {
            const empty = { userName: "", tenantID: "" }
            setSearch(empty)
            setApplied(empty)
            setPageIndex(1)
          }}
        >
          Reset
        </Button>
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>User Name</TableHead>
              <TableHead>Nickname</TableHead>
              <TableHead>Email</TableHead>
              <TableHead>Tenant ID</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={6} className="py-8 text-center text-muted-foreground">
                  Loading...
                </TableCell>
              </TableRow>
            ) : users.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="py-8 text-center text-muted-foreground">
                  No data
                </TableCell>
              </TableRow>
            ) : (
              users.map((u) => (
                <TableRow key={u.id}>
                  <TableCell className="font-medium">{u.userName}</TableCell>
                  <TableCell>{u.nickname || "-"}</TableCell>
                  <TableCell>{u.email || "-"}</TableCell>
                  <TableCell className="font-mono text-xs">{u.tenantID || "-"}</TableCell>
                  <TableCell>
                    {u.enable ? (
                      <Badge>Enabled</Badge>
                    ) : (
                      <Badge variant="secondary">Disabled</Badge>
                    )}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="inline-flex gap-1">
                      <Button
                        size="icon"
                        variant="ghost"
                        title="Edit"
                        onClick={() => openEdit(u)}
                      >
                        <Pencil />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        title={u.enable ? "Disable" : "Enable"}
                        onClick={() =>
                          enableMutation.mutate({ id: u.id, enable: !u.enable })
                        }
                      >
                        <Power />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        title="Reset Password"
                        onClick={() => {
                          setResetTarget(u)
                          setResetPwd("")
                        }}
                      >
                        <KeyRound />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        title="Delete"
                        onClick={() => {
                          if (window.confirm("Delete " + u.userName + "?"))
                            deleteMutation.mutate(u.id)
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

      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>Total {total}</span>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={pageIndex <= 1}
            onClick={() => setPageIndex((p) => Math.max(1, p - 1))}
          >
            Prev
          </Button>
          <span>{pageIndex} / {totalPages}</span>
          <Button
            variant="outline"
            size="sm"
            disabled={pageIndex >= totalPages}
            onClick={() => setPageIndex((p) => Math.min(totalPages, p + 1))}
          >
            Next
          </Button>
        </div>
      </div>

      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>{form.id ? "Edit User" : "Add User"}</DialogTitle>
          </DialogHeader>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1">
              <Label>User Name</Label>
              <Input
                value={form.userName}
                disabled={!!form.id}
                onChange={(e) => setForm({ ...form, userName: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>Nickname</Label>
              <Input
                value={form.nickname}
                onChange={(e) => setForm({ ...form, nickname: e.target.value })}
              />
            </div>
            {!form.id && (
              <div className="space-y-1">
                <Label>Password</Label>
                <Input
                  type="password"
                  value={form.password}
                  onChange={(e) => setForm({ ...form, password: e.target.value })}
                />
              </div>
            )}
            <div className="space-y-1">
              <Label>Email</Label>
              <Input
                value={form.email}
                onChange={(e) => setForm({ ...form, email: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>Mobile</Label>
              <Input
                value={form.mobile}
                onChange={(e) => setForm({ ...form, mobile: e.target.value })}
              />
            </div>
            <div className="flex items-center gap-2 pt-6">
              <Switch
                checked={form.enable}
                onCheckedChange={(v) => setForm({ ...form, enable: v })}
              />
              <Label>Enabled</Label>
            </div>
          </div>

          <div className="space-y-2">
            <Label>Roles</Label>
            <div className="max-h-36 space-y-1 overflow-y-auto rounded-md border p-2">
              {(roles ?? []).length === 0 ? (
                <div className="py-2 text-center text-sm text-muted-foreground">
                  No roles available
                </div>
              ) : (
                (roles ?? []).map((r: any) => (
                  <label key={r.id} className="flex cursor-pointer items-center gap-2 rounded px-1 py-1 hover:bg-accent">
                    <Checkbox
                      checked={form.roleIDs.includes(r.id)}
                      onCheckedChange={(v) => toggleRole(r.id, v === true)}
                    />
                    <span className="text-sm">{r.name}</span>
                  </label>
                ))
              )}
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setEditOpen(false)}>
              Cancel
            </Button>
            <Button onClick={submitForm} disabled={addMutation.isPending || updateMutation.isPending}>
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={!!resetTarget}
        onOpenChange={(o) => {
          if (!o) { setResetTarget(null); setResetPwd("") }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Reset Password - {resetTarget?.userName}</DialogTitle>
          </DialogHeader>
          <div className="space-y-1">
            <Label>New password (leave empty to generate randomly)</Label>
            <Input
              value={resetPwd}
              placeholder="Leave empty for random"
              onChange={(e) => setResetPwd(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setResetTarget(null); setResetPwd("") }}>
              Cancel
            </Button>
            <Button
              onClick={() => { if (resetTarget) resetMutation.mutate({ id: resetTarget.id, password: resetPwd }) }}
              disabled={resetMutation.isPending}
            >
              Reset
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
