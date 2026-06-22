import { lazy, Suspense } from "react"
import { createBrowserRouter, type RouteObject } from "react-router-dom"
import AppShell from "@/components/layout/AppShell"
import { ProtectedRoute } from "@/lib/auth"

const Login = lazy(() => import("@/pages/Login"))
const Dashboard = lazy(() => import("@/pages/Dashboard"))
const Users = lazy(() => import("@/pages/Users"))
const Roles = lazy(() => import("@/pages/Roles"))
const Tenants = lazy(() => import("@/pages/Tenants"))
const Gateway = lazy(() => import("@/pages/Gateway"))
const AIKeys = lazy(() => import("@/pages/AIKeys"))
const Usage = lazy(() => import("@/pages/Usage"))
const Sessions = lazy(() => import("@/pages/Sessions"))
const Audit = lazy(() => import("@/pages/Audit"))
const LiveAudit = lazy(() => import("@/pages/LiveAudit"))
const Security = lazy(() => import("@/pages/Security"))
const OAuthClients = lazy(() => import("@/pages/OAuthClients"))
const Scim = lazy(() => import("@/pages/Scim"))
const SystemConfig = lazy(() => import("@/pages/SystemConfig"))
const ApiTester = lazy(() => import("@/pages/ApiTester"))

function SuspenseWrapper({ children }: { children: React.ReactNode }) {
  return (
    <Suspense
      fallback={
        <div className="flex h-full items-center justify-center">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
        </div>
      }
    >
      {children}
    </Suspense>
  )
}

const routes: RouteObject[] = [
  {
    path: "/login",
    element: (
      <SuspenseWrapper>
        <Login />
      </SuspenseWrapper>
    ),
  },
  {
    path: "/",
    element: (
      <ProtectedRoute>
        <AppShell />
      </ProtectedRoute>
    ),
    children: [
      {
        index: true,
        element: (
          <SuspenseWrapper>
            <Dashboard />
          </SuspenseWrapper>
        ),
      },
      {
        path: "users",
        element: (
          <SuspenseWrapper>
            <Users />
          </SuspenseWrapper>
        ),
      },
      {
        path: "roles",
        element: (
          <SuspenseWrapper>
            <Roles />
          </SuspenseWrapper>
        ),
      },
      {
        path: "tenants",
        element: (
          <SuspenseWrapper>
            <Tenants />
          </SuspenseWrapper>
        ),
      },
      {
        path: "gateway",
        element: (
          <SuspenseWrapper>
            <Gateway />
          </SuspenseWrapper>
        ),
      },
      {
        path: "aikeys",
        element: (
          <SuspenseWrapper>
            <AIKeys />
          </SuspenseWrapper>
        ),
      },
      {
        path: "usage",
        element: (
          <SuspenseWrapper>
            <Usage />
          </SuspenseWrapper>
        ),
      },
      {
        path: "sessions",
        element: (
          <SuspenseWrapper>
            <Sessions />
          </SuspenseWrapper>
        ),
      },
      {
        path: "audit",
        element: (
          <SuspenseWrapper>
            <Audit />
          </SuspenseWrapper>
        ),
      },
      {
        path: "liveaudit",
        element: (
          <SuspenseWrapper>
            <LiveAudit />
          </SuspenseWrapper>
        ),
      },
      {
        path: "security",
        element: (
          <SuspenseWrapper>
            <Security />
          </SuspenseWrapper>
        ),
      },
      {
        path: "oauth",
        element: (
          <SuspenseWrapper>
            <OAuthClients />
          </SuspenseWrapper>
        ),
      },
      {
        path: "scim",
        element: (
          <SuspenseWrapper>
            <Scim />
          </SuspenseWrapper>
        ),
      },
      {
        path: "config",
        element: (
          <SuspenseWrapper>
            <SystemConfig />
          </SuspenseWrapper>
        ),
      },
      {
        path: "tester",
        element: (
          <SuspenseWrapper>
            <ApiTester />
          </SuspenseWrapper>
        ),
      },
    ],
  },
]

export const router = createBrowserRouter(routes)
