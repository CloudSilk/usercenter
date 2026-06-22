import { useState, useEffect } from "react"
import { Link, Outlet, useLocation } from "react-router-dom"
import { useAuth } from "@/lib/auth"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { cn } from "@/lib/utils"
import {
  BarChart3,
  User,
  School,
  Building2,
  Rocket,
  Key,
  TrendingUp,
  Cable,
  FileText,
  Activity,
  Shield,
  Lock,
  Share2,
  Settings,
  Play,
  Moon,
  Sun,
  LogOut,
  Menu,
  X,
  ChevronDown,
} from "lucide-react"

interface NavItem {
  label: string
  path: string
  icon: React.ReactNode
}

interface NavGroup {
  label: string
  items: NavItem[]
}

const navGroups: NavGroup[] = [
  {
    label: "组织管理",
    items: [
      { label: "Dashboard", path: "/", icon: <BarChart3 className="h-4 w-4" /> },
      { label: "Users", path: "/users", icon: <User className="h-4 w-4" /> },
      { label: "Roles", path: "/roles", icon: <School className="h-4 w-4" /> },
      { label: "Tenants", path: "/tenants", icon: <Building2 className="h-4 w-4" /> },
    ],
  },
  {
    label: "AI 能力",
    items: [
      { label: "Gateway", path: "/gateway", icon: <Rocket className="h-4 w-4" /> },
      { label: "AI Keys", path: "/aikeys", icon: <Key className="h-4 w-4" /> },
      { label: "Usage", path: "/usage", icon: <TrendingUp className="h-4 w-4" /> },
    ],
  },
  {
    label: "安全审计",
    items: [
      { label: "Sessions", path: "/sessions", icon: <Cable className="h-4 w-4" /> },
      { label: "Audit", path: "/audit", icon: <FileText className="h-4 w-4" /> },
      { label: "Live Monitor", path: "/liveaudit", icon: <Activity className="h-4 w-4" /> },
      { label: "Security", path: "/security", icon: <Shield className="h-4 w-4" /> },
    ],
  },
  {
    label: "系统集成",
    items: [
      { label: "OAuth", path: "/oauth", icon: <Lock className="h-4 w-4" /> },
      { label: "SCIM", path: "/scim", icon: <Share2 className="h-4 w-4" /> },
      { label: "Config", path: "/config", icon: <Settings className="h-4 w-4" /> },
      { label: "API Tester", path: "/tester", icon: <Play className="h-4 w-4" /> },
    ],
  },
]

function Sidebar({ collapsed, onToggle }: { collapsed: boolean; onToggle: () => void }) {
  const location = useLocation()
  const { profile } = useAuth()
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>(() => {
    // Auto-expand the group containing the current path
    const init: Record<string, boolean> = {}
    for (const group of navGroups) {
      init[group.label] = group.items.some((item) => item.path === location.pathname)
    }
    return init
  })

  const toggleGroup = (label: string) => {
    setExpandedGroups((prev) => ({ ...prev, [label]: !prev[label] }))
  }

  const isActive = (path: string) => {
    if (path === "/") return location.pathname === "/"
    return location.pathname.startsWith(path)
  }

  return (
    <>
      {/* Mobile overlay */}
      {!collapsed && (
        <div
          className="fixed inset-0 z-40 bg-black/50 md:hidden"
          onClick={onToggle}
        />
      )}

      <aside
        className={cn(
          "fixed left-0 top-0 z-50 flex h-full flex-col border-r bg-background transition-transform duration-200 md:static md:z-auto md:translate-x-0",
          collapsed ? "-translate-x-full" : "translate-x-0",
          "w-60",
        )}
      >
        {/* Brand */}
        <div className="flex h-14 items-center justify-between px-4">
          <Link to="/" className="flex items-center gap-2 font-semibold">
            <Shield className="h-5 w-5 text-primary" />
            <span>UserCenter</span>
          </Link>
          <button
            onClick={onToggle}
            className="rounded-md p-1 hover:bg-accent md:hidden"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <Separator />

        {/* Navigation */}
        <nav className="flex-1 overflow-y-auto p-2">
          {navGroups.map((group) => (
            <div key={group.label} className="mb-1">
              <button
                onClick={() => toggleGroup(group.label)}
                className="flex w-full items-center justify-between rounded-md px-2 py-1.5 text-xs font-medium text-muted-foreground hover:bg-accent"
              >
                <span>{group.label}</span>
                <ChevronDown
                  className={cn(
                    "h-3 w-3 transition-transform",
                    expandedGroups[group.label] && "rotate-180",
                  )}
                />
              </button>
              {expandedGroups[group.label] && (
                <div className="ml-1 space-y-0.5">
                  {group.items.map((item) => (
                    <Link
                      key={item.path}
                      to={item.path}
                      onClick={() => {
                        // Close mobile sidebar on navigate
                        if (window.innerWidth < 768) onToggle()
                      }}
                      className={cn(
                        "flex items-center gap-2.5 rounded-md px-2 py-1.5 text-sm transition-colors",
                        isActive(item.path)
                          ? "bg-primary text-primary-foreground"
                          : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                      )}
                    >
                      {item.icon}
                      <span>{item.label}</span>
                    </Link>
                  ))}
                </div>
              )}
            </div>
          ))}
        </nav>

        {/* User badge */}
        {profile && (
          <div className="border-t p-3">
            <div className="flex items-center gap-2">
              <div className="flex h-7 w-7 items-center justify-center rounded-full bg-primary text-xs font-medium text-primary-foreground">
                {profile.user.nickname?.charAt(0) || profile.user.userName?.charAt(0) || "U"}
              </div>
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium">
                  {profile.user.nickname || profile.user.userName}
                </p>
                <p className="truncate text-xs text-muted-foreground">
                  {profile.tenant?.name || ""}
                </p>
              </div>
            </div>
          </div>
        )}
      </aside>
    </>
  )
}

export default function AppShell() {
  const { logout } = useAuth()
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [dark, setDark] = useState(() => {
    if (typeof window !== "undefined") {
      const stored = localStorage.getItem("uc_dark_mode")
      if (stored !== null) return stored === "true"
      return window.matchMedia("(prefers-color-scheme: dark)").matches
    }
    return false
  })

  useEffect(() => {
    if (dark) {
      document.documentElement.classList.add("dark")
    } else {
      document.documentElement.classList.remove("dark")
    }
    localStorage.setItem("uc_dark_mode", String(dark))
  }, [dark])

  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar collapsed={!sidebarOpen} onToggle={() => setSidebarOpen(!sidebarOpen)} />

      <div className="flex flex-1 flex-col overflow-hidden">
        {/* Header */}
        <header className="flex h-14 items-center gap-3 border-b bg-background px-4">
          <button
            onClick={() => setSidebarOpen(!sidebarOpen)}
            className="rounded-md p-1 hover:bg-accent"
          >
            <Menu className="h-5 w-5" />
          </button>

          <div className="flex-1" />

          <button
            onClick={() => setDark(!dark)}
            className="rounded-md p-1.5 hover:bg-accent"
            title={dark ? "Switch to light mode" : "Switch to dark mode"}
          >
            {dark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
          </button>

          <Button variant="ghost" size="sm" onClick={logout}>
            <LogOut className="mr-1 h-4 w-4" />
            Logout
          </Button>
        </header>

        {/* Main content */}
        <main className="flex-1 overflow-auto bg-muted/30 p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
