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
  const [selectedArch, setSelectedArch] = useState<string>(() => {
    const ua = navigator.userAgent.toLowerCase()
    if (ua.includes('mac') || ua.includes('darwin')) return 'arm64'
    return 'amd64'
  })

  useEffect(() => {
    backend.get<ReleasesResponse>('/automation/releases')
      .then(data => setReleases(data.releases || []))
      .catch(err => setError(err.message))
      .finally(() => setLoading(false))
  }, [])

  const changeLang = (e: React.ChangeEvent<HTMLSelectElement>) => {
    i18n.changeLanguage(e.target.value)
  }

  const os = selectedOS === 'windows' ? 'linux' : selectedOS
  const latestUrl = `${(backend.baseUrl || '')}/automation/latest?os=${os}&arch=${selectedArch}`
  const installCmd = `curl -Lo bitswan ${latestUrl} && chmod +x bitswan`
  const pathCmd = `sudo mv bitswan /usr/local/bin/`

  return (
    <div className="app">
      <nav className="navbar">
        <svg className="nav-logo" viewBox="0 0 663.4 154.8" xmlns="http://www.w3.org/2000/svg" aria-label="BitSwan">
          <path fill="currentColor" d="M612.6,77.7c5.8-5.8,12.4-8.7,19.8-8.7c6.1,0,10.8,1,14,3s4.3,3.7,4.3,7.3v38h12.6V78.6c0-5.6-1.8-9.8-5.4-12.9c-5.6-4.7-13.2-7.1-22.7-7.1c-8.6,0-16.2,3.3-22.7,9.9v-8.6H600v57.5h12.6V77.7z M583.2,117.3V59.8h-12.6V68c-7-6-15.8-9.4-25-9.5c-9,0-16.7,2.6-23,7.9c-3.5,3.1-5.4,7.3-5.4,12.9v18.5c0,5.6,1.8,9.8,5.4,12.9c6.1,5.2,13.8,7.8,23,7.8c9.2,0.2,18.2-3.2,25-9.4v8.1L583.2,117.3z M570.6,98.4c-2.7,3.2-6.2,5.6-10.1,7c-3.8,1.8-8,2.8-12.2,2.9c-4.8,0-9.6-1.3-13.8-3.7c-3.5-2-4.7-3.8-4.7-7.4V80.1c0-3.8,1.1-5.5,4.7-7.4c4.2-2.4,8.9-3.6,13.8-3.6c4.2,0.1,8.4,1,12.2,2.7c4.5,1.8,7.8,4.1,10.1,7V98.4z M491.7,117.3l18.1-57.5h-13.1l-14.2,47.3h-3l-15-47.3h-13.4l-16.3,47.3H432l-13.9-47.3h-13.6l18.2,57.5H443l14.5-42.9l14,42.9H491.7z M360.5,118.4c12.2,0,21.1-1.2,26.5-3.6c6.4-3,9.6-7.6,9.6-13.6v-6.1c0.2-3.8-1.4-7.5-4.2-10c-2.6-2.4-7.2-4.6-14-6.3l-20.1-5.4c-5.6-1.4-9.2-2.8-10.6-4s-2.4-3.3-2.4-6c0-2.9,1-4.9,3-6.2c2.6-1.7,8.4-2.6,17.1-2.6c9.1-0.1,18.1,0.8,27,2.7V46.8c-8.5-1.6-17-2.4-25.6-2.3c-12.7,0-21.8,1.7-27.1,5.3c-4.7,3.1-7,7.1-7,12v5.3c-0.1,3.6,1.3,7,3.9,9.5c3.1,2.9,8.4,5.4,16,7.3l19.2,5.2c9.3,2.3,12.1,4.5,12.1,9.5c0,3.6-1.1,6-3.3,7.2c-3.3,1.7-9.8,2.6-19.4,2.6c-9.5,0.1-18.9-0.9-28.2-2.7v10.7C341.9,117.8,351.1,118.5,360.5,118.4 M323.3,106.7c-4.6,1.3-9.3,1.9-14.1,1.8c-4.7,0-8.4-0.9-11-2.9c-2.4-1.8-3.1-4-3.1-8.6V69.6h28.2v-9.9h-28.1V45.1h-12.6v52.8c0,7.8,1.4,12,5.8,15.7c4.2,3.3,10.6,4.9,19.4,4.9c6.8,0,11.9-0.7,15.6-2.2L323.3,106.7z M266,59.7h-12.6v57.5H266V59.7z M266,36.5h-12.6v13.1H266V36.5z M213,117.3c11.8,0,18-1.3,22.7-5.3c4.5-3.8,6.1-6.5,6.1-12.4v-5c0-6-2.9-10.3-8.7-12.9c-0.9-0.5-1.6-0.8-1.9-0.9l0.4-0.2c5.4-2.2,8-6.3,8-12.4V63c0-5.5-1.4-8.5-5.1-11.8c-4.4-3.7-11.8-5.5-22.4-5.5h-36.3v71.6H213z M215.2,85.9c5,0,8.6,0.8,10.8,2.5s3.3,4.5,3.3,8.4s-1.3,6.5-3.7,8.2c-2.2,1.6-6.3,2.4-12.3,2.4h-25.1V85.9H215.2z M211.2,55.7c6.8,0,11.1,0.9,13.3,2.9c1.7,1.7,2.6,4.2,2.6,7.7c0,3.7-0.9,6.2-2.8,7.7c-2.1,1.5-5,2.3-8.9,2.3h-27.2V55.7H211.2z"/>
          <path fill="currentColor" d="M0,104.5V5l59.9,50L10.3,92.8C6,96,2.5,100,0,104.5z M90.7,80.6l-21.3,18c-7.1,6.2-10.9,14.5-10.9,24c0,8.6,3.4,16.7,9.4,22.8c6.1,6.1,14.2,9.5,22.8,9.5s16.7-3.4,22.8-9.5c6.1-6.1,9.4-14.2,9.4-22.8s-3.3-16.7-9.4-22.7L90.7,80.6z M118.5,15.8l-25,19.5l0,0L13.1,96.6C4.9,102.6,0,112.3,0,122.5c0,8.6,3.4,16.7,9.4,22.8c6.1,6.1,14.2,9.5,22.8,9.5h40.4c-2.9-1.6-5.6-3.7-8.1-6.1c-7-7-10.8-16.3-10.8-26.1c0-10.7,4.4-20.5,12.5-27.6l46-38.7c6.8-5.8,10.8-14.9,10.8-24C123,26.4,121.5,20.8,118.5,15.8z M57.5,0l36.1,29.3L115.7,12c-0.5-0.6-1.3-1.5-2.3-2.7C107,1.6,97.5,0,90.8,0H57.5z"/>
        </svg>
        <div className="nav-right">
          <select className="lang-select" value={i18n.language} onChange={changeLang}>
            <option value="en">{t('language.en')}</option>
            <option value="cs">{t('language.cs')}</option>
          </select>
          <button className="theme-toggle" onClick={toggleTheme}>
            {theme === 'dark' ? '\u2600\uFE0F' : '\uD83C\uDF19'}
          </button>
        </div>
      </nav>

      <header className="hero">
        <h1>{t('app.title')}</h1>
        <p className="hero-subtitle">{t('app.description')}</p>
      </header>

      <section className="install-section" id="install">
        <div className="section-header">
          <h2>{t('install.title')}</h2>
          <p className="section-desc">{t('install.prereq')}</p>
        </div>

        <div className="install-card">
          <div className="install-card-header">
            <h3>{t('install.step1Title')}</h3>
            <p>{t('install.step1Desc')}</p>
          </div>
          <div className="platform-picker">
            <div className="picker-group">
              <label>{t('install.os')}</label>
              <div className="picker-options">
                {(['linux', 'darwin', 'windows'] as const).map(os => (
                  <button
                    key={os}
                    className={`picker-btn ${selectedOS === os ? 'active' : ''}`}
                    onClick={() => { setSelectedOS(os); setSelectedArch(os === 'darwin' ? 'arm64' : 'amd64') }}
                  >
                    {os === 'darwin' ? 'macOS' : os === 'linux' ? 'Linux' : 'Windows WSL'}
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
          <CopyBlock text={pathCmd} />
        </div>

        <div className="install-card">
          <div className="install-card-header">
            <h3>{t('install.step2Title')}</h3>
            <p>{t('install.step2Desc')}</p>
          </div>
          <CopyBlock text="bitswan automation-server-daemon init" />
        </div>

        <div className="install-card">
          <div className="install-card-header">
            <h3>{t('install.step3Title')}</h3>
            <p>{t('install.step3Desc')}</p>
          </div>
          <div className="init-options">
            <div className="init-option">
              <span className="init-label">SaaS</span>
              <p className="init-desc">{t('install.saasDesc')} <a href="https://aoc.bitswan.ai" target="_blank" rel="noopener noreferrer">aoc.bitswan.ai</a></p>
            </div>
            <div className="init-option">
              <span className="init-label">{t('install.onPremPublic')}</span>
              <CopyBlock text="bitswan workspace init --domain=my-workspace.bitswan.io my-workspace" />
            </div>
            <div className="init-option">
              <span className="init-label">{t('install.onPremInternal')}</span>
              <CopyBlock text="bitswan workspace init --domain=my-workspace.my-domain.local --certs-dir=/etc/certs my-workspace" />
            </div>
            <div className="init-option">
              <span className="init-label">{t('install.local')}</span>
              <CopyBlock text="bitswan workspace init --local dev-workspace" />
            </div>
            <div className="init-option">
              <span className="init-label">{t('install.withGit')}</span>
              <CopyBlock text="bitswan workspace init --remote=git@github.com:<your-name>/<your-repo>.git my-workspace" />
            </div>
          </div>
        </div>
      </section>

      {error && <p className="error-text">{t('error.prefix')}: {error}</p>}

      {loading ? (
        <p className="loading-text">{t('common.loading')}</p>
      ) : releases.length > 0 ? (
        <section className="releases-section" id="releases">
          <div className="section-header">
            <h2>{t('releases.title')}</h2>
          </div>
          <div className="releases">
            {releases.map(release => (
              <div key={release.tag_name} className="release-card">
                <div className="release-header">
                  <div className="release-title-row">
                    <h3>{release.release_name || release.tag_name}</h3>
                    <span className="release-version">{release.tag_name}</span>
                  </div>
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
                    <div className="download-grid">
                      {release.assets.map(asset => (
                        <a
                          key={asset.name}
                          href={backend.getDownloadUrl(release.tag_name, asset.name)}
                          className="download-item"
                          download
                        >
                          <svg className="download-icon" width="18" height="18" viewBox="0 0 18 18" fill="none">
                            <path d="M9 2v10m0 0l-3.5-3.5M9 12l3.5-3.5M3 14h12" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
                          </svg>
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
        </section>
      ) : (
        <p className="no-releases">{t('releases.empty')}</p>
      )}

      <footer className="footer">
        <a href="https://bitswan.ai" target="_blank" rel="noopener noreferrer">
          <img src="/bitswan.svg" alt="BitSwan" className="footer-logo" />
          Powered by BitSwan
        </a>
      </footer>
    </div>
  )
}

export default App
