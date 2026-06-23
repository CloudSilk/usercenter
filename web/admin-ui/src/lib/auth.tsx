import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react"
import { Navigate, useLocation } from "react-router-dom"
import { api, clearToken, getToken, setToken } from "@/lib/api"
import type { SocialProvider, UserProfile } from "@/lib/types"

interface AuthContextValue {
  token: string | null
  profile: UserProfile | null
  socialProviders: SocialProvider[]
  login: (userName: string, password: string) => Promise<void>
  logout: () => Promise<void>
  loading: boolean
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error("useAuth must be used within AuthProvider")
  }
  return ctx
}

async function loadProfile(): Promise<UserProfile> {
  // Backend may return flat {id, userName, nickname, ...} or nested {user:{...}, tenant:{...}}
  // Normalize both shapes into the expected UserProfile format.
  const raw = await api.get<Record<string, unknown>>("/api/core/auth/user/profile")
  const user = raw as any
  if (user && user.user) {
    // Already nested — return as-is
    return user as UserProfile
  }
  // Flat shape from backend — wrap into expected structure
  return {
    user: {
      id: user.id,
      userName: user.userName,
      nickname: user.nickname,
      email: user.email,
      mobile: user.mobile,
      avatar: user.avatar,
      realName: user.realName,
      gender: user.gender,
      type: user.type,
      group: user.group,
      idCard: user.idCard,
    },
    tenant: { id: user.tenantID, name: "" },
    roles: [],
    funcCodes: [],
    menus: [],
  } as UserProfile
}

async function loadSocialProviders(): Promise<SocialProvider[]> {
  try {
    const data = await api.get<SocialProvider[]>("/api/social/providers")
    return Array.isArray(data) ? data : []
  } catch {
    return []
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setTokenState] = useState<string | null>(getToken)
  const [profile, setProfile] = useState<UserProfile | null>(null)
  const [socialProviders, setSocialProviders] = useState<SocialProvider[]>([])
  const [loading, setLoading] = useState(true)

  // On mount: check for social_token, load profile + social providers
  useEffect(() => {
    async function init() {
      // Check for social_token in URL
      const params = new URLSearchParams(window.location.search)
      const socialToken = params.get("social_token")
      if (socialToken) {
        setToken(socialToken)
        setTokenState(socialToken)
        // Remove social_token from URL
        params.delete("social_token")
        const newUrl = params.toString()
          ? `${window.location.pathname}?${params.toString()}`
          : window.location.pathname
        window.history.replaceState(null, "", newUrl)
      }

      const storedToken = getToken()
      if (storedToken) {
        try {
          const [userProfile, providers] = await Promise.all([
            loadProfile(),
            loadSocialProviders(),
          ])
          setProfile(userProfile)
          setSocialProviders(providers)
        } catch {
          clearToken()
          setTokenState(null)
        }
      }
      setLoading(false)
    }
    init()
  }, [])

  const login = useCallback(async (userName: string, password: string) => {
    // Backend returns { code: 20000, data: "<jwt_string>" } — api.request extracts data field
    const jwtToken = await api.post<string>("/api/core/auth/user/login", {
      userName,
      password,
    })
    setToken(jwtToken)
    setTokenState(jwtToken)

    const [userProfile, providers] = await Promise.all([
      loadProfile(),
      loadSocialProviders(),
    ])
    setProfile(userProfile)
    setSocialProviders(providers)
  }, [])

  const logout = useCallback(async () => {
    try {
      await api.post("/api/core/auth/user/logout")
    } catch {
      // ignore logout errors
    }
    clearToken()
    setTokenState(null)
    setProfile(null)
    setSocialProviders([])
  }, [])

  const value = useMemo(
    () => ({ token, profile, socialProviders, login, logout, loading }),
    [token, profile, socialProviders, login, logout, loading],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function ProtectedRoute({ children }: { children: ReactNode }) {
  const { token, loading } = useAuth()
  const location = useLocation()

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    )
  }

  if (!token) {
    return <Navigate to="/login" state={{ from: location }} replace />
  }

  return <>{children}</>
}
