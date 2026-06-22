import { AuthProvider } from "@/lib/auth"
import { QueryProvider } from "@/lib/query"
import { router } from "@/routes"
import { RouterProvider } from "react-router-dom"
import { Toaster } from "sonner"

export default function App() {
  return (
    <AuthProvider>
      <QueryProvider>
        <RouterProvider router={router} />
        <Toaster position="top-right" richColors />
      </QueryProvider>
    </AuthProvider>
  )
}
