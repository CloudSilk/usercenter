import { useEffect, useState } from "react"
import { useNavigate, useLocation } from "react-router-dom"
import { useAuth } from "@/lib/auth"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Shield } from "lucide-react"
import { toast } from "sonner"

export default function Login() {
  const { login, socialProviders, token } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [userName, setUserName] = useState("")
  const [password, setPassword] = useState("")
  const [loading, setLoading] = useState(false)

  // If already logged in, redirect to home
  useEffect(() => {
    if (token) {
      const from = (location.state as { from?: Location })?.from?.pathname || "/"
      navigate(from, { replace: true })
    }
  }, [token, navigate, location])

  // social_token is handled by AuthProvider on mount

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!userName.trim() || !password.trim()) {
      toast.error("Please enter username and password")
      return
    }
    setLoading(true)
    try {
      await login(userName, password)
      const from = (location.state as { from?: Location })?.from?.pathname || "/"
      navigate(from, { replace: true })
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "Login failed"
      toast.error(message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900 p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <div className="mx-auto mb-2 flex h-12 w-12 items-center justify-center rounded-full bg-primary">
            <Shield className="h-6 w-6 text-primary-foreground" />
          </div>
          <CardTitle className="text-xl">UserCenter Admin</CardTitle>
          <CardDescription>Sign in to your account</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="userName">Username</Label>
              <Input
                id="userName"
                placeholder="Enter username"
                value={userName}
                onChange={(e) => setUserName(e.target.value)}
                autoFocus
                autoComplete="username"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                placeholder="Enter password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
              />
            </div>
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? "Signing in..." : "Sign In"}
            </Button>
          </form>

          {socialProviders.length > 0 && (
            <>
              <div className="relative my-6">
                <div className="absolute inset-0 flex items-center">
                  <div className="w-full border-t" />
                </div>
                <div className="relative flex justify-center text-xs uppercase">
                  <span className="bg-card px-2 text-muted-foreground">
                    Or continue with
                  </span>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-2">
                {socialProviders.map((sp) => (
                  <Button
                    key={sp.name}
                    variant="outline"
                    onClick={() => {
                      window.location.href = sp.authURL
                    }}
                  >
                    {sp.icon && (
                      <img src={sp.icon} alt="" className="mr-2 h-4 w-4" />
                    )}
                    {sp.name}
                  </Button>
                ))}
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
