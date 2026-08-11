import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { api } from '../services/api'
import type { Branding } from '../types'
const fallback: Branding = { systemTitle: 'FluentHub', schoolDisplayName: 'Language School', primaryColor: '#4f46e5', secondaryColor: '#0f172a', accentColor: '#f59e0b', welcomeText: 'Welcome to your learning journey.' }
const BrandingContext = createContext(fallback)
export function BrandingProvider({ children }: { children: ReactNode }) { const [branding, setBranding] = useState(fallback); useEffect(() => { api.branding().then(value => { setBranding(value); document.title = value.systemTitle; document.documentElement.style.setProperty('--brand-primary', value.primaryColor); document.documentElement.style.setProperty('--brand-secondary', value.secondaryColor); document.documentElement.style.setProperty('--brand-accent', value.accentColor) }).catch(() => undefined) }, []); return <BrandingContext.Provider value={branding}>{children}</BrandingContext.Provider> }
export const useBranding = () => useContext(BrandingContext)
