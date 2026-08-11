export type Role = 'administrator' | 'teacher' | 'student' | 'operator'
export interface CurrentUser { id: string; schoolID: string; name: string; email: string; roles: string[]; permissions: string[] }
export interface Branding { systemTitle: string; schoolDisplayName: string; primaryColor: string; secondaryColor: string; accentColor: string; welcomeText: string; logoLightStorageKey?: string; logoDarkStorageKey?: string; faviconStorageKey?: string }
export interface ApiError { error: { code: string; message: string; requestId: string } }
