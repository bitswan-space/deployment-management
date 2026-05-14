// BitSwan API client for the external app (no authentication required)

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

// Get the backend URL (public endpoints live under /public)
export const getBackendUrl = (): string | null => {
  const base = getAutomationUrl('backend')
  return base ? `${base}/public` : null
}

// Backend API client (no auth — external app)
class BackendClient {
  baseUrl: string | null

  constructor(baseUrl: string | null = null) {
    this.baseUrl = baseUrl || getBackendUrl()
  }

  async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    if (!this.baseUrl) {
      throw new Error('Backend URL not configured')
    }

    const url = `${this.baseUrl}${path}`
    const response = await fetch(url, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...(options.headers as Record<string, string>),
      },
    })

    if (!response.ok) {
      const body = await response.json().catch(() => ({}))
      throw new Error(body.detail || `HTTP ${response.status}: ${response.statusText}`)
    }

    return response.json()
  }

  get<T>(path: string): Promise<T> {
    return this.request<T>(path)
  }

  getDownloadUrl(tag: string, asset: string): string {
    return `${this.baseUrl}/automation/download/${encodeURIComponent(tag)}/${encodeURIComponent(asset)}`
  }
}

// Singleton instance
export const backend = new BackendClient()

export default BackendClient
