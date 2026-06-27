import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Pencil, Plus, Trash2 } from "lucide-react"

import { api } from "@/lib/api"
import type { Role } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
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

interface RoleListResponse {
  data: Role[]
  records: number
}

interface RoleFormState {
  id: string
  name: string
  description: string
  defaultRouter: string
  public: boolean
}

const emptyForm: RoleFormState = {
  id: "",
  name: "",
  description: "",
  defaultRouter: "",
  public: false,
}

export default function RoleManagement() {
  const qc = useQueryClient()

  const [search, setSearch] = useState("")
  const [applied, setApplied] = useState("")
  const [pageIndex, setPageIndex] = useState(1)
  const pageSize = 10

  const [editOpen, setEditOpen] = useState(false)
  const [form, setForm] = useState<RoleFormState>(emptyForm)

  const listKey = ["role-management", applied, pageIndex, pageSize]

  const { data, isLoading } = useQuery<RoleListResponse>({
    queryKey: listKey,
    queryFn: () =>
      api.get<RoleListResponse>("/api/core/auth/role/query", {
        pageIndex,
        pageSize,
        name: applied || undefined,
      }),
  })

  const roles = data?.data ?? []
  const total = data?.records ?? 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  const addMutation = useMutation({
    mutationFn: (body: any) => api.post("/api/core/auth/role/add", body),
    onSuccess: () => {
      toast.success("Role created")
      setEditOpen(false)
      qc.invalidateQueries({ queryKey: ["role-management"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const updateMutation = useMutation({
    mutationFn: (body: any) => api.put("/api/core/auth/role/update", body),
    onSuccess: () => {
      toast.success("Role updated")
      setEditOpen(false)
      qc.invalidateQueries({ queryKey: ["role-management"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.del("/api/core/auth/role/delete", { id }),
    onSuccess: () => {
      toast.success("Role deleted")
      qc.invalidateQueries({ queryKey: ["role-management"] })
    },
    onError: (e: Error) => toast.error(e.message),
  })

  function openAdd() {
    setForm({ ...emptyForm })
    setEditOpen(true)
  }

  function openEdit(r: Role) {
    setForm({
      id: r.id,
      name: r.name ?? "",
      description: r.description ?? "",
      defaultRouter: (r as any).defaultRouter ?? "",
      public: !!(r as any).public,
    })
    setEditOpen(true)
  }

  function submitForm() {
    if (!form.name.trim()) {
      toast.error("Please enter role name")
      return
    }
    if (form.id) {
      updateMutation.mutate(form)
    } else {
      const { id: _id, ...body } = form
      void _id
      addMutation.mutate(body)
    }
  }

  return (
    <div className="space-y-4 p-4 md:p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-bold">Role Management</h1>
        <Button size="sm" onClick={openAdd}>
          <Plus /> Add Role
        </Button>
      </div>

      <div className="flex flex-wrap items-end gap-2">
        <div className="space-y-1">
          <Label className="text-xs">Name</Label>
          <Input
            className="h-8 w-56"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                setApplied(search)
                setPageIndex(1)
              }
            }}
          />
        </div>
        <Button
          size="sm"
          onClick={() => {
            setApplied(search)
            setPageIndex(1)
          }}
        >
          Search
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={() => {
            setSearch("")
            setApplied("")
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
              <TableHead>Name</TableHead>
              <TableHead>Description</TableHead>
              <TableHead>Default Router</TableHead>
              <TableHead>Public</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={5} className="py-8 text-center text-muted-foreground">
                  Loading...
                </TableCell>
              </TableRow>
            ) : roles.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="py-8 text-center text-muted-foreground">
                  No data
                </TableCell>
              </TableRow>
            ) : (
              roles.map((r) => (
                <TableRow key={r.id}>
                  <TableCell className="font-medium">{r.name}</TableCell>
                  <TableCell className="max-w-xs truncate text-muted-foreground">
                    {r.description || "-"}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">
                    {(r as any).defaultRouter || "-"}
                  </TableCell>
                  <TableCell>
                    {(r as any).public ? (
                      <span className="text-green-600 font-medium">Yes</span>
                    ) : (
                      <span className="text-muted-foreground">No</span>
                    )}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="inline-flex gap-1">
                      <Button size="icon" variant="ghost" title="Edit" onClick={() => openEdit(r)}>
                        <Pencil />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        title="Delete"
                        disabled={(r as any).canDel === false}
                        onClick={() => {
                          if (window.confirm("Delete role " + r.name + "?"))
                            deleteMutation.mutate(r.id)
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
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{form.id ? "Edit Role" : "Add Role"}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-1">
              <Label>Name</Label>
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>Description</Label>
              <Input
                value={form.description}
                onChange={(e) => setForm({ ...form, description: e.target.value })}
              />
            </div>
            <div className="space-y-1">
              <Label>Default Router</Label>
              <Input
                value={form.defaultRouter}
                onChange={(e) => setForm({ ...form, defaultRouter: e.target.value })}
              />
            </div>
            <div className="flex items-center gap-2">
              <Switch
                checked={form.public}
                onCheckedChange={(v) => setForm({ ...form, public: v })}
              />
              <Label>Public</Label>
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
    </div>
  )
}
