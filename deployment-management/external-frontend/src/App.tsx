import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { backend } from './api'
import './App.css'

interface ReleaseAsset {
  name: string
  size: number
  content_type: string
}

interface PublishedRelease {
  tag_name: string
  release_name: string
  body: string
  assets: ReleaseAsset[]
  published_at: string
}

interface ReleasesResponse {
  releases: PublishedRelease[]
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

function CopyBlock({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = () => {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  return (
    <div className="code-block-wrapper" onClick={handleCopy}>
      <pre className="code-block">{text}</pre>
      <span className={`copy-btn ${copied ? 'copied' : ''}`}>
        {copied ? (
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M3 8.5l3 3 7-7" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/></svg>
        ) : (
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><rect x="5" y="5" width="9" height="9" rx="1.5" stroke="currentColor" strokeWidth="1.5"/><path d="M3 11V3a1.5 1.5 0 011.5-1.5H11" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"/></svg>
        )}
      </span>
    </div>
  )
}

function App() {
  const { t, i18n } = useTranslation()
  const { theme, toggle: toggleTheme } = useTheme()
  const [releases, setReleases] = useState<PublishedRelease[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [selectedOS, setSelectedOS] = useState<string>(() => {
    const ua = navigator.userAgent.toLowerCase()
    if (ua.includes('mac') || ua.includes('darwin')) return 'darwin'
    if (ua.includes('win')) return 'windows'
    return 'linux'
  })
  const [selectedArch, setSelectedArch] = useState<string>('amd64')

  useEffect(() => {
    backend.get<ReleasesResponse>('/automation/releases')
      .then(data => setReleases(data.releases || []))
      .catch(err => setError(err.message))
      .finally(() => setLoading(false))
  }, [])

  const changeLang = (e: React.ChangeEvent<HTMLSelectElement>) => {
    i18n.changeLanguage(e.target.value)
  }

  const latestUrl = `${(backend.baseUrl || '')}/automation/latest?os=${selectedOS}&arch=${selectedArch}`
  const installCmd = selectedOS === 'windows'
    ? `curl -Lo bitswan.exe "${latestUrl}"`
    : `curl -Lo bitswan ${latestUrl} && chmod +x bitswan`
  const pathCmd = selectedOS === 'windows'
    ? null
    : `sudo mv bitswan /usr/local/bin/`

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

      <div className="install-section">
        <h2>{t('install.title')}</h2>
        <p className="install-prereq">{t('install.prereq')}</p>
        <div className="install-steps">
          <div className="install-step">
            <h3>{t('install.step1Title')}</h3>
            <p>{t('install.step1Desc')}</p>
            <div className="platform-picker">
              <div className="picker-group">
                <label>{t('install.os')}</label>
                <div className="picker-options">
                  {(['linux', 'darwin', 'windows'] as const).map(os => (
                    <button
                      key={os}
                      className={`picker-btn ${selectedOS === os ? 'active' : ''}`}
                      onClick={() => setSelectedOS(os)}
                    >
                      {os === 'darwin' ? 'macOS' : os === 'linux' ? 'Linux' : 'Windows'}
                    </button>
                  ))}
                </div>
              </div>
              <div className="picker-group">
                <label>{t('install.arch')}</label>
                <div className="picker-options">
                  {(['amd64', 'arm64'] as const).map(arch => (
                    <button
                      key={arch}
                      className={`picker-btn ${selectedArch === arch ? 'active' : ''}`}
                      onClick={() => setSelectedArch(arch)}
                    >
                      {arch === 'amd64' ? 'x86_64' : 'ARM64'}
                    </button>
                  ))}
                </div>
              </div>
            </div>
            <CopyBlock text={installCmd} />
            {pathCmd && <CopyBlock text={pathCmd} />}
          </div>

          <div className="install-step">
            <h3>{t('install.step2Title')}</h3>
            <p>{t('install.step2Desc')}</p>
            <strong>{t('install.saas')}</strong>
            <CopyBlock text="bitswan workspace init my-workspace" />
            <strong>{t('install.onPremPublic')}</strong>
            <CopyBlock text="bitswan workspace init --domain=my-workspace.bitswan.io my-workspace" />
            <strong>{t('install.onPremInternal')}</strong>
            <CopyBlock text="bitswan workspace init --domain=my-workspace.my-domain.local --certs-dir=/etc/certs my-workspace" />
            <strong>{t('install.local')}</strong>
            <CopyBlock text="bitswan workspace init --local dev-workspace" />
            <strong>{t('install.withGit')}</strong>
            <CopyBlock text="bitswan workspace init --remote=git@github.com:<your-name>/<your-repo>.git my-workspace" />
          </div>
        </div>
      </div>

      {error && <p className="error-text">{t('error.prefix')}: {error}</p>}

      {loading ? (
        <p>{t('common.loading')}</p>
      ) : releases.length > 0 ? (
        <div className="releases-section">
          <h2>{t('releases.title')}</h2>
          <div className="releases">
            {releases.map(release => (
              <div key={release.tag_name} className="release-card">
                <div className="release-header">
                  <h3>{release.release_name || release.tag_name}</h3>
                  <span className="release-version">{release.tag_name}</span>
                  <span className="release-date">{new Date(release.published_at).toLocaleDateString()}</span>
                </div>

                {release.body && (
                  <div className="release-body">
                    <p>{release.body}</p>
                  </div>
                )}

                {release.assets && release.assets.length > 0 && (
                  <div className="download-section">
                    <h4>{t('downloads.title')}</h4>
                    <div className="download-list">
                      {release.assets.map(asset => (
                        <a
                          key={asset.name}
                          href={backend.getDownloadUrl(release.tag_name, asset.name)}
                          className="download-item"
                          download
                        >
                          <span className="download-icon">&#x2B07;</span>
                          <span className="download-name">{asset.name}</span>
                          <span className="download-size">{formatSize(asset.size)}</span>
                        </a>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      ) : (
        <p className="no-releases">{t('releases.empty')}</p>
      )}

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
