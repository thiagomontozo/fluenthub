import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { api } from '../services/api'
import type { CurrentUser, Role } from '../types'
interface AuthState { user: CurrentUser | null; loading: boolean; login(email: string, password: string): Promise<Role>; logout(): Promise<void> }
const Context = createContext<AuthState | null>(null)
function roleOf(user: CurrentUser): Role { const value = user.roles[0]?.toLowerCase(); return value === 'teacher' || value === 'student' || value === 'operator' ? value : 'administrator' }
export function AuthProvider({ children }: { children: ReactNode }) { const [user, setUser] = useState<CurrentUser | null>(null); const [loading, setLoading] = useState(true); useEffect(() => { api.me().then(setUser).catch(() => setUser(null)).finally(() => setLoading(false)) }, []); const value: AuthState = { user, loading, login: async (email, password) => { const authenticated = await api.login(email, password); setUser(authenticated); return roleOf(authenticated) }, logout: async () => { await api.logout(); setUser(null) } }; return <Context.Provider value={value}>{children}</Context.Provider> }
export function useAuth() { const value = useContext(Context); if (!value) throw new Error('AuthProvider is missing'); return value }
