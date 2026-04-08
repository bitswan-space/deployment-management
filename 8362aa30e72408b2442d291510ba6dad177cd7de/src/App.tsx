import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { backend, getUserInfo, getAccessToken, getTokenInfo, type UserInfo, type TokenInfo } from './api'
import './App.css'

interface DockerTag {
  name: string
  full_size: number
  last_updated: string
  digest: string
}

interface TagGroup {
  digest: string
  tags: DockerTag[]
  is_in_prod: boolean
  display_name: string
}

interface RepoStatusResponse {
  repository: string
  staging_repo: string
  staging_groups: TagGroup[]
  production_tags: DockerTag[]
}

interface GitHubAsset {
  id: number
  name: string
  size: number
  content_type: string
  browser_download_url: string
}

interface GitHubRelease {
  id: number
  tag_name: string
  name: string
  body: string
  draft: boolean
  prerelease: boolean
  created_at: string
  published_at: string
  assets: GitHubAsset[]
  is_published: boolean
  published_by?: string
  local_published_at?: string
}

interface ReleasesResponse {
  releases: GitHubRelease[]
}

function useTheme() {
  const [theme, setTheme] = useState<'light' | 'dark'>(() => {
    const stored = localStorage.getItem('theme')
    if (stored === 'light' || stored === 'dark') return stored
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  })

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
    localStorage.setItem('theme', theme)
  }, [theme])

  const toggle = () => setTheme(t => t === 'dark' ? 'light' : 'dark')
  return { theme, toggle }
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i]
}

function DockerRepoSection({ repo, label }: { repo: string; label: string }) {
  const { t } = useTranslation()
  const [status, setStatus] = useState<RepoStatusResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [promoting, setPromoting] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)

  const fetchStatus = async () => {
    try {
      setLoading(true)
      const data = await backend.get<RepoStatusResponse>(`/docker/status?repo=${repo}`)
      setStatus(data)
      setError(null)
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { fetchStatus() }, [repo])

  const handlePromote = async (tag: string) => {
    if (!confirm(t('docker.confirmPromote', { tag, repo }))) return
    setPromoting(tag)
    setActionError(null)
    try {
      await backend.post('/docker/promote', { repository: repo, tag })
      await fetchStatus()
    } catch (err: any) {
      setActionError(`${t('docker.promoteFailed')}: ${err.message}`)
    } finally {
      setPromoting(null)
    }
  }

  if (loading) return <div className="card"><h2>{label}</h2><p>{t('common.loading')}</p></div>
  if (error) return <div className="card"><h2>{label}</h2><p className="error-text">{error}</p></div>
  if (!status) return null

  return (
    <div className="card">
      <h2>{label}</h2>
      {actionError && (
        <div className="error-banner">
          <span>{actionError}</span>
          <button className="error-dismiss" onClick={() => setActionError(null)}>&times;</button>
        </div>
      )}
      <p className="repo-subtitle">{t('docker.stagingRepo')}: <code>{status.staging_repo}</code> | {t('docker.productionRepo')}: <code>{status.repository}</code></p>

      <h3>{t('docker.stagingImages')}</h3>
      {status.staging_groups && status.staging_groups.length > 0 ? (
        <div className="tag-list">
          {status.staging_groups.map(group => (
            <div key={group.digest} className={`tag-group ${group.is_in_prod ? 'in-prod' : ''}`}>
              <div className="tag-group-header">
                <span className="tag-display-name">{group.display_name}</span>
                {group.is_in_prod && <span className="badge badge-prod">{t('docker.inProduction')}</span>}
              </div>
              <div className="tag-group-tags">
                {group.tags.map(tag => (
                  <span key={tag.name} className="tag-chip" title={tag.digest}>
                    {tag.name}
                  </span>
                ))}
              </div>
              <div className="tag-group-meta">
                <span>{formatSize(group.tags[0]?.full_size || 0)}</span>
                <span>{new Date(group.tags[0]?.last_updated).toLocaleString()}</span>
              </div>
              {!group.is_in_prod && (
                <button
                  className="promote-btn"
                  onClick={() => handlePromote(group.display_name)}
                  disabled={promoting !== null}
                >
                  {promoting === group.display_name ? t('docker.promoting') : t('docker.promoteToProduction')}
                </button>
              )}
            </div>
          ))}
        </div>
      ) : (
        <p>{t('docker.noStagingImages')}</p>
      )}

      <h3>{t('docker.productionImages')}</h3>
      {status.production_tags && status.production_tags.length > 0 ? (
        <div className="tag-list">
          {status.production_tags.map(tag => (
            <div key={tag.name} className="tag-item">
              <span className="tag-chip prod-tag">{tag.name}</span>
              <span className="tag-meta">{formatSize(tag.full_size)} | {new Date(tag.last_updated).toLocaleString()}</span>
            </div>
          ))}
        </div>
      ) : (
        <p>{t('docker.noProductionImages')}</p>
      )}
    </div>
  )
}

function AutomationSection() {
  const { t } = useTranslation()
  const [releases, setReleases] = useState<GitHubRelease[]>([])
  const [loading, setLoading] = useState(true)
  const [publishing, setPublishing] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)

  const fetchReleases = async () => {
    try {
      setLoading(true)
      const data = await backend.get<ReleasesResponse>('/automation/releases')
      setReleases(data.releases || [])
      setError(null)
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { fetchReleases() }, [])

  const handlePublish = async (releaseId: number) => {
    if (!confirm(t('automation.confirmPublish'))) return
    setPublishing(releaseId)
    setActionError(null)
    try {
      await backend.post('/automation/publish', { release_id: releaseId })
      await fetchReleases()
    } catch (err: any) {
      setActionError(`${t('automation.publishFailed')}: ${err.message}`)
    } finally {
      setPublishing(null)
    }
  }

  const handleUnpublish = async (releaseId: number) => {
    if (!confirm(t('automation.confirmUnpublish'))) return
    setActionError(null)
    try {
      await backend.post('/automation/unpublish', { release_id: releaseId })
      await fetchReleases()
    } catch (err: any) {
      setActionError(`${t('automation.unpublishFailed')}: ${err.message}`)
    }
  }

  if (loading) return <div className="card"><h2>{t('automation.title')}</h2><p>{t('common.loading')}</p></div>
  if (error) return <div className="card"><h2>{t('automation.title')}</h2><p className="error-text">{error}</p></div>

  return (
    <div className="card">
      <h2>{t('automation.title')}</h2>
      {actionError && (
        <div className="error-banner">
          <span>{actionError}</span>
          <button className="error-dismiss" onClick={() => setActionError(null)}>&times;</button>
        </div>
      )}
      <p className="section-description">{t('automation.description')}</p>

      {releases.length > 0 ? (
        <div className="release-list">
          {releases.map(release => (
            <div key={release.id} className={`release-item ${release.is_published ? 'published' : ''}`}>
              <div className="release-header">
                <div className="release-title">
                  <strong>{release.name || release.tag_name}</strong>
                  <span className="release-tag">{release.tag_name}</span>
                  {release.prerelease && <span className="badge badge-pre">pre-release</span>}
                  {release.is_published && <span className="badge badge-published">{t('automation.published')}</span>}
                </div>
                <span className="release-date">{new Date(release.published_at).toLocaleDateString()}</span>
              </div>

              {release.body && (
                <p className="release-body">{release.body.slice(0, 200)}{release.body.length > 200 ? '...' : ''}</p>
              )}

              {release.assets && release.assets.length > 0 && (
                <div className="release-assets">
                  {release.assets.map(asset => (
                    <span key={asset.id || asset.name} className="asset-chip">
                      {asset.name} ({formatSize(asset.size)})
                    </span>
                  ))}
                </div>
              )}

              {release.is_published && release.published_by && (
                <p className="release-published-info">
                  {t('automation.publishedBy')}: {release.published_by}
                  {release.local_published_at && ` | ${new Date(release.local_published_at).toLocaleString()}`}
                </p>
              )}

              <div className="release-actions">
                {!release.is_published ? (
                  <button
                    className="publish-btn"
                    onClick={() => handlePublish(release.id)}
                    disabled={publishing !== null}
                  >
                    {publishing === release.id ? t('automation.publishing') : t('automation.publish')}
                  </button>
                ) : (
                  <button
                    className="unpublish-btn"
                    onClick={() => handleUnpublish(release.id)}
                  >
                    {t('automation.unpublish')}
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      ) : (
        <p>{t('automation.noReleases')}</p>
      )}
    </div>
  )
}

function App() {
  const { t, i18n } = useTranslation()
  const { theme, toggle: toggleTheme } = useTheme()
  const [user, setUser] = useState<UserInfo | null>(null)
  const [tokenInfo, setTokenInfo] = useState<TokenInfo | null>(null)
  const [activeTab, setActiveTab] = useState<'docker' | 'automation'>('docker')

  useEffect(() => {
    getUserInfo()
      .then(setUser)
      .catch(err => console.error('Failed to fetch user info:', err))

    getAccessToken()
      .then(() => setTokenInfo(getTokenInfo()))
      .catch(() => {})
  }, [])

  useEffect(() => {
    const interval = setInterval(() => {
      const info = getTokenInfo()
      if (info) setTokenInfo(info)
    }, 1000)
    return () => clearInterval(interval)
  }, [])

  const changeLang = (e: React.ChangeEvent<HTMLSelectElement>) => {
    i18n.changeLanguage(e.target.value)
  }

  return (
    <div className="app">
      <div className="toolbar">
        <select className="lang-select" value={i18n.language} onChange={changeLang}>
          <option value="en">{t('language.en')}</option>
          <option value="cs">{t('language.cs')}</option>
        </select>
        <button onClick={toggleTheme}>
          {theme === 'dark' ? '\u2600\uFE0F' : '\uD83C\uDF19'}
        </button>
      </div>

      <h1>{t('app.title')}</h1>
      <p className="description">{t('app.description')}</p>

      {user && (
        <div className="user-bar">
          <span>{user.preferredUsername || user.email}</span>
          {tokenInfo && (
            <span className={tokenInfo.ttlSeconds <= 0 ? 'expired' : ''}>
              TTL: {tokenInfo.ttlSeconds}s
            </span>
          )}
          <button className="sign-out" onClick={() => window.location.href = '/oauth2/sign_out'}>
            {t('userInfo.signOut')}
          </button>
        </div>
      )}

      <div className="tabs">
        <button
          className={`tab ${activeTab === 'docker' ? 'active' : ''}`}
          onClick={() => setActiveTab('docker')}
        >
          {t('tabs.docker')}
        </button>
        <button
          className={`tab ${activeTab === 'automation' ? 'active' : ''}`}
          onClick={() => setActiveTab('automation')}
        >
          {t('tabs.automation')}
        </button>
      </div>

      {activeTab === 'docker' && (
        <div className="docker-grid">
          <DockerRepoSection repo="bitswan-editor" label={t('docker.editorTitle')} />
          <DockerRepoSection repo="gitops" label={t('docker.gitopsTitle')} />
        </div>
      )}

      {activeTab === 'automation' && <AutomationSection />}

      <footer className="powered-by">
        <a href="https://bitswan.ai" target="_blank" rel="noopener noreferrer">
          <img src="/bitswan.svg" alt="BitSwan" className="powered-by-logo" />
          Powered by BitSwan
        </a>
      </footer>
    </div>
  )
}

export default App
