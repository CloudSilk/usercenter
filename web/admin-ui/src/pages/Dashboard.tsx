import { useEffect, useState, useCallback } from "react"
import { apiGet } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import type { DashboardStats } from "@/lib/types"
import {
  Users,
  School,
  KeyRound,
  Coins,
  RefreshCw,
} from "lucide-react"

export default function Dashboard() {
  const { profile } = useAuth()
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [autoRefresh, setAutoRefresh] = useState(false)

  const fetchStats = useCallback(async () => {
    try {
      const data = await apiGet<DashboardStats>("/admin/api/stats")
      setStats(data)
    } catch {
      // ignore
    }
  }, [])

  useEffect(() => {
    fetchStats()
  }, [fetchStats])

  useEffect(() => {
    if (!autoRefresh) return
    const interval = setInterval(fetchStats, 10000)
    return () => clearInterval(interval)
  }, [autoRefresh, fetchStats])

  const statCards = stats
    ? [
        { label: "Users", value: stats.userCount, icon: <Users className="h-5 w-5 text-muted-foreground" /> },
        { label: "Roles", value: stats.roleCount, icon: <School className="h-5 w-5 text-muted-foreground" /> },
        { label: "Sessions", value: stats.sessionCount, icon: <KeyRound className="h-5 w-5 text-muted-foreground" /> },
        { label: "Today Tokens", value: stats.todayTokens, icon: <Coins className="h-5 w-5 text-muted-foreground" /> },
      ]
    : []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Dashboard</h1>
        <Button
          variant={autoRefresh ? "default" : "outline"}
          size="sm"
          onClick={() => setAutoRefresh(!autoRefresh)}
        >
          <RefreshCw className={`mr-2 h-4 w-4 ${autoRefresh ? "animate-spin" : ""}`} />
          {autoRefresh ? "Auto-refresh ON" : "Auto-refresh OFF"}
        </Button>
      </div>

      {/* Stats */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        {statCards.map((s) => (
          <Card key={s.label}>
            <CardHeader className="flex flex-row items-center justify-between pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                {s.label}
              </CardTitle>
              {s.icon}
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {s.value.toLocaleString()}
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* User Profile Card */}
      {profile && (
        <Card>
          <CardHeader>
            <CardTitle>Profile</CardTitle>
          </CardHeader>
          <CardContent>
            <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
              <dt className="font-medium text-muted-foreground">Username</dt>
              <dd>{profile.user.userName}</dd>
              <dt className="font-medium text-muted-foreground">Nickname</dt>
              <dd>{profile.user.nickname || "-"}</dd>
              <dt className="font-medium text-muted-foreground">Email</dt>
              <dd>{profile.user.email || "-"}</dd>
              <dt className="font-medium text-muted-foreground">Mobile</dt>
              <dd>{profile.user.mobile || "-"}</dd>
              <dt className="font-medium text-muted-foreground">Tenant</dt>
              <dd>{profile.tenant?.name || "-"}</dd>
              <dt className="font-medium text-muted-foreground">Roles</dt>
              <dd>{profile.roles?.map((r) => r.name).join(", ") || "-"}</dd>
            </dl>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
