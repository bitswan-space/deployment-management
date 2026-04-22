// BitSwan API client for connecting to backend automations

declare global {
  interface Window {
    __BITSWAN_CONFIG__?: BitswanConfig | null
  }
}

interface BitswanConfig {
  workspaceName?: string
  deploymentId?: string
  stage?: string
  domain?: string
  urlTemplate?: string
}

const getConfig = (): BitswanConfig => window.__BITSWAN_CONFIG__ || {}

// Build URL for a named automation using the URL template.
export const getAutomationUrl = (name: string): string | null => {
  const config = getConfig()
  if (config.urlTemplate) {
    return config.urlTemplate.replace('{name}', name).replace(/\/+$/, '')
  }
  return null
}

// Get the backend URL (internal endpoints live under /internal)
export const getBackendUrl = (): string | null => {
  const base = getAutomationUrl('backend')
  return base ? `${base}/internal` : null
}

// Access token management for authenticated backend calls.
let cachedToken: string | null = null

async function fetchAccessToken(): Promise<string> {
  const response = await fetch('/oauth2/auth')
  if (!response.ok) {
    throw new Error(`Failed to fetch access token: ${response.status}`)
  }
  const token = response.headers.get('X-Auth-Request-Access-Token')
  if (!token) {
    throw new Error('No access token in oauth2-proxy response')
  }
  cachedToken = token
  return token
}

export async function getAccessToken(): Promise<string> {
  if (cachedToken) return cachedToken
  return fetchAccessToken()
}

export interface TokenInfo {
  expiresAt: Date
  issuedAt: Date
  ttlSeconds: number
}

export function getTokenInfo(): TokenInfo | null {
  if (!cachedToken) return null
  try {
    const payload = JSON.parse(atob(cachedToken.split('.')[1]))
    const now = Math.floor(Date.now() / 1000)
    return {
      expiresAt: new Date(payload.exp * 1000),
      issuedAt: new Date(payload.iat * 1000),
      ttlSeconds: payload.exp - now,
    }
  } catch {
    return null
  }
}

// Backend API client
class BackendClient {
  baseUrl: string | null

  constructor(baseUrl: string | null = null) {
    this.baseUrl = baseUrl || getBackendUrl()
  }

  async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    if (!this.baseUrl) {
      throw new Error('Backend URL not configured')
    }

    const token = await getAccessToken()
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string>),
      'Authorization': `Bearer ${token}`,
    }

    const url = `${this.baseUrl}${path}`
    let response = await fetch(url, { ...options, headers })

    // If 401, token may have expired — refresh and retry once
    if (response.status === 401) {
      cachedToken = null
      const newToken = await fetchAccessToken()
      headers['Authorization'] = `Bearer ${newToken}`
      response = await fetch(url, { ...options, headers })
    }

    if (!response.ok) {
      const body = await response.json().catch(() => ({}))
      throw new Error(body.detail || `HTTP ${response.status}: ${response.statusText}`)
    }

    return response.json()
  }

  get<T>(path: string): Promise<T> {
    return this.request<T>(path)
  }

  post<T>(path: string, data?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: 'POST',
      body: data ? JSON.stringify(data) : undefined,
    })
  }

  delete<T>(path: string): Promise<T> {
    return this.request<T>(path, { method: 'DELETE' })
  }
}

// Singleton instance
export const backend = new BackendClient()

// OAuth2 Proxy user info
export interface UserInfo {
  email?: string
  user?: string
  groups?: string[]
  preferredUsername?: string
}

export async function getUserInfo(): Promise<UserInfo> {
  const response = await fetch('/oauth2/userinfo')
  if (!response.ok) {
    throw new Error(`Failed to fetch user info: ${response.status}`)
  }
  const data = await response.json()
  return {
    email: data.email,
    user: data.user,
    groups: data.groups,
    preferredUsername: data.preferredUsername || data.preferred_username,
  }
}

export default BackendClient
