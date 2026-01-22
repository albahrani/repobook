(function () {
  const elNav = document.getElementById('nav')
  const elViewer = document.getElementById('viewer')
  const elToc = document.getElementById('toc')
  const elCrumb = document.getElementById('crumb')
  const elStatus = document.getElementById('status')

  let tree = null
  let currentPath = ''
  let currentMTime = 0

  function setStatus(msg) {
    elStatus.textContent = msg || ''
  }

  function esc(s) {
    return String(s).replace(/[&<>\"']/g, (c) => ({
      '&': '&amp;',
      '<': '&lt;',
      '>': '&gt;',
      '"': '&quot;',
      "'": '&#39;'
    }[c]))
  }

  function getRoutePath() {
    if (location.pathname.startsWith('/file/')) {
      return decodeURIComponent(location.pathname.slice('/file/'.length))
    }
    return ''
  }

  async function fetchJSON(url) {
    const res = await fetch(url, { cache: 'no-store' })
    if (!res.ok) throw new Error(await res.text())
    return res.json()
  }

  function renderTreeNode(node) {
    if (node.type === 'file') {
      const active = node.path === currentPath ? ' is-active' : ''
      return (
        `<div class="nav-item file${active}">` +
          `<a class="nav-link" href="/file/${encodeURIComponent(node.path)}">${esc(node.name)}</a>` +
        `</div>`
      )
    }

    const dirId = 'dir-' + btoa(unescape(encodeURIComponent(node.path || 'root'))).replace(/=+$/g, '')
    const children = (node.children || []).map(renderTreeNode).join('')
    return (
      `<details class="nav-dir" id="${dirId}" open>` +
        `<summary class="nav-dir-title">${esc(node.name || 'root')}</summary>` +
        `<div class="nav-dir-children">${children}</div>` +
      `</details>`
    )
  }

  function renderTree() {
    if (!tree) return
    elNav.innerHTML = renderTreeNode(tree)
  }

  function renderTOC(toc) {
    if (!toc || !toc.length) {
      elToc.innerHTML = '<div class="toc-empty">No headings</div>'
      return
    }
    elToc.innerHTML = toc.map((it) => {
      const pad = Math.max(0, Math.min(5, it.level - 1))
      const href = it.id ? `#${encodeURIComponent(it.id)}` : '#'
      return `<a class="toc-item lvl-${it.level}" style="padding-left:${pad * 12}px" href="${href}">${esc(it.title)}</a>`
    }).join('')
  }

  function setCrumb(p) {
    if (!p) {
      elCrumb.textContent = ''
      return
    }
    elCrumb.textContent = p
  }

  async function loadDoc(relPath, opts) {
    const anchor = (opts && opts.anchor) || ''
    setStatus('Loading…')
    const data = await fetchJSON(`/api/render?path=${encodeURIComponent(relPath)}`)
    currentPath = data.path
    currentMTime = data.mtime || 0
    document.title = `repobook • ${data.title || data.path}`
    setCrumb(data.path)
    elViewer.innerHTML = `<article class="markdown-body">${data.html}</article>`
    renderTOC(data.toc || [])
    renderTree()
    setStatus('')

    const target = anchor || location.hash
    if (target && target.startsWith('#')) {
      // goldmark auto heading IDs are plain strings; they might contain spaces.
      const id = decodeURIComponent(target.slice(1))
      const el = document.getElementById(id)
      if (el) {
        setTimeout(() => el.scrollIntoView({ block: 'start' }), 0)
      }
    }
  }

  async function ensureHome() {
    const home = await fetchJSON('/api/home')
    if (!home.path) {
      elViewer.innerHTML = '<div class="empty">No README.md found at repo root.</div>'
      elToc.innerHTML = ''
      return
    }
    navigate(`/file/${encodeURIComponent(home.path)}`, true)
  }

  function navigate(urlPath, replace) {
    if (replace) {
      history.replaceState({}, '', urlPath)
    } else {
      history.pushState({}, '', urlPath)
    }
    route()
  }

  async function route() {
    const p = getRoutePath()
    if (!p) {
      await ensureHome()
      return
    }
    await loadDoc(p)
  }

  function setupLinkInterception() {
    document.addEventListener('click', (e) => {
      const a = e.target && e.target.closest ? e.target.closest('a') : null
      if (!a) return
      const href = a.getAttribute('href')
      if (!href) return
      if (href.startsWith('#')) return

      // Same-origin SPA navigation.
      try {
        const u = new URL(href, location.origin)
        if (u.origin === location.origin && u.pathname.startsWith('/file/')) {
          e.preventDefault()
          navigate(u.pathname + u.hash, false)
        }
      } catch (_) {
        // ignore
      }
    })

    window.addEventListener('popstate', () => {
      route()
    })
  }

  function setupLiveUpdates() {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    const ws = new WebSocket(`${proto}://${location.host}/ws`)
    ws.onmessage = (msg) => {
      let ev
      try { ev = JSON.parse(msg.data) } catch (_) { return }
      if (!ev || !ev.type) return
      if (ev.type === 'tree-updated') {
        loadTree().catch(() => {})
      }
      if (ev.type === 'file-changed' && ev.path && ev.path === currentPath) {
        // Avoid spamming reloads when multiple events fire.
        setTimeout(() => {
          loadDoc(currentPath, { anchor: location.hash }).catch(() => {})
        }, 100)
      }
    }
  }

  async function loadTree() {
    tree = await fetchJSON('/api/tree')
    renderTree()
  }

  async function boot() {
    setupLinkInterception()
    await loadTree()
    await route()
    setupLiveUpdates()
  }

  boot().catch((err) => {
    setStatus('')
    elViewer.innerHTML = `<pre class="error">${esc(err && err.message ? err.message : String(err))}</pre>`
  })
})()
