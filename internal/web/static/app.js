(function () {
  const elNav = document.getElementById('nav')
  const elViewer = document.getElementById('viewer')
  const elToc = document.getElementById('toc')
  const elCrumb = document.getElementById('crumb')
  const elStatus = document.getElementById('status')
	const elSearch = document.getElementById('search')
	const elResults = document.getElementById('results')
	const elSearchMeta = document.getElementById('searchMeta')
	const elNavToggle = document.getElementById('navToggle')

	let tree = null
	let currentPath = ''
	let currentMTime = 0
	let scrollSpyDisconnect = null
	let searchTimer = null
	let lastQuery = ''
	const openDirPaths = new Set()
	let navCollapsed = false

	function syncNavToggle() {
		if (!elNavToggle) return
		elNavToggle.textContent = navCollapsed ? 'Expand' : 'Collapse'
	}

	function isPathInDir(filePath, dirPath) {
		if (!dirPath) return true
		return filePath === dirPath || filePath.startsWith(dirPath + '/')
	}

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
					`<a class="nav-link" href="/file/${encodeURI(node.path)}">${esc(node.name)}</a>` +
				`</div>`
			)
		}

		const active = isPathInDir(currentPath, node.path) ? ' is-active' : ''
		const shouldOpen = !navCollapsed || !node.path || active || openDirPaths.has(node.path)
		const openAttr = shouldOpen ? ' open' : ''
		const dirId = 'dir-' + btoa(unescape(encodeURIComponent(node.path || 'root'))).replace(/=+$/g, '')
		const children = (node.children || []).map(renderTreeNode).join('')
		return (
			`<details class="nav-dir${active}" id="${dirId}" data-path="${esc(node.path || '')}"${openAttr}>` +
				`<summary class="nav-dir-title">${esc(node.name || 'root')}</summary>` +
				`<div class="nav-dir-children">${children}</div>` +
			`</details>`
		)
	}

	function renderTree() {
		if (!tree) return
		elNav.innerHTML = renderTreeNode(tree)

		// Persist manual open/close.
		elNav.querySelectorAll('details.nav-dir').forEach((d) => {
			d.addEventListener('toggle', () => {
				const p = d.getAttribute('data-path') || ''
				if (!p) return
				if (d.open) openDirPaths.add(p)
				else openDirPaths.delete(p)
			})
		})

		// Ensure the active entry is visible.
		setTimeout(() => {
			const active = elNav.querySelector('.nav-item.is-active .nav-link')
			if (active && active.scrollIntoView) active.scrollIntoView({ block: 'nearest' })
		}, 0)
	}

	function setupNavToggle() {
		if (!elNavToggle) return
		try {
			navCollapsed = localStorage.getItem('repobook.navCollapsed') === '1'
		} catch (_) {
			navCollapsed = false
		}
		syncNavToggle()
		elNavToggle.addEventListener('click', () => {
			navCollapsed = !navCollapsed
			if (navCollapsed) openDirPaths.clear()
			try {
				localStorage.setItem('repobook.navCollapsed', navCollapsed ? '1' : '0')
			} catch (_) {
				// ignore
			}
			syncNavToggle()
			renderTree()
		})
	}

	function setSearchMeta(msg) {
		if (!elSearchMeta) return
		elSearchMeta.textContent = msg || ''
	}

	function showResults(show) {
		if (!elResults) return
		if (show) {
			elNav.hidden = true
			elResults.hidden = false
		} else {
			elResults.hidden = true
			elNav.hidden = false
		}
	}

	function renderResults(data) {
		if (!elResults) return
		if (!data || !data.results || !data.results.length) {
			elResults.innerHTML = '<div class="toc-empty">No results</div>'
			return
		}
		elResults.innerHTML = data.results.map((r) => {
			const href = `/file/${encodeURI(r.path)}`
			return (
				`<a class="result" href="${href}">` +
					`<div class="result-top">` +
						`<div class="result-path">${esc(r.path)}</div>` +
						`<div class="result-line">L${esc(r.line)}</div>` +
					`</div>` +
					`<div class="result-preview">${esc(r.preview)}</div>` +
				`</a>`
			)
		}).join('')
		if (data.truncated) {
			elResults.innerHTML += '<div class="toc-empty">Results truncated</div>'
		}
	}

	async function runSearch(q) {
		q = (q || '').trim()
		lastQuery = q
		if (!q) {
			setSearchMeta('')
			showResults(false)
			return
		}
		setSearchMeta('Searching…')
		showResults(true)
		try {
			const data = await fetchJSON(`/api/search?q=${encodeURIComponent(q)}`)
			if (lastQuery !== q) return
			renderResults(data)
			setSearchMeta(`${data.results.length}${data.truncated ? '+' : ''} results`)
		} catch (err) {
			if (lastQuery !== q) return
			if (elResults) {
				elResults.innerHTML = `<pre class="error">${esc(err && err.message ? err.message : String(err))}</pre>`
			}
			setSearchMeta('Search failed')
		}
	}

	function renderTOC(toc) {
		if (!toc || !toc.length) {
			elToc.innerHTML = '<div class="toc-empty">No headings</div>'
			return
		}
		elToc.innerHTML = toc.map((it) => {
			const pad = Math.max(0, Math.min(5, it.level - 1))
			const href = it.id ? `#${encodeURIComponent(it.id)}` : '#'
			const data = it.id ? ` data-id="${esc(it.id)}"` : ''
			return `<a class="toc-item lvl-${it.level}" style="padding-left:${pad * 12}px" href="${href}"${data}>${esc(it.title)}</a>`
		}).join('')
	}

	function setupTOCBehavior() {
		elToc.addEventListener('click', (e) => {
			const a = e.target && e.target.closest ? e.target.closest('a.toc-item') : null
			if (!a) return
			const id = a.getAttribute('data-id')
			if (!id) return
			e.preventDefault()
			// Update URL hash without triggering a full route.
			history.replaceState({}, '', `${location.pathname}#${encodeURIComponent(id)}`)
			const el = document.getElementById(id)
			if (el) el.scrollIntoView({ block: 'start' })
		})
	}

	function setupScrollSpy() {
		if (scrollSpyDisconnect) {
			scrollSpyDisconnect()
			scrollSpyDisconnect = null
		}
		const headings = elViewer.querySelectorAll('h1[id],h2[id],h3[id],h4[id],h5[id],h6[id]')
		if (!headings.length) return

		const linksByID = new Map()
		elToc.querySelectorAll('a.toc-item[data-id]').forEach((a) => {
			linksByID.set(a.getAttribute('data-id'), a)
		})

		function setActive(id) {
			elToc.querySelectorAll('a.toc-item.is-active').forEach((x) => x.classList.remove('is-active'))
			const a = linksByID.get(id)
			if (a) a.classList.add('is-active')
		}

		const io = new IntersectionObserver((entries) => {
			// Choose the entry closest to the top that is intersecting.
			let best = null
			for (const ent of entries) {
				if (!ent.isIntersecting) continue
				if (!best || ent.boundingClientRect.top < best.boundingClientRect.top) {
					best = ent
				}
			}
			if (best && best.target && best.target.id) setActive(best.target.id)
		}, {
			root: elViewer,
			rootMargin: '0px 0px -70% 0px',
			threshold: [0, 1],
		})

		headings.forEach((h) => io.observe(h))
		// Set initial active.
		setTimeout(() => {
			for (const h of headings) {
				if (h.getBoundingClientRect().top >= 0) {
					setActive(h.id)
					break
				}
			}
		}, 0)

		scrollSpyDisconnect = () => io.disconnect()
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
		setupScrollSpy()
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
				if (elSearch && elSearch.value) {
					elSearch.value = ''
					runSearch('')
				}
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

	function setupSearch() {
		if (!elSearch) return
		elSearch.addEventListener('input', () => {
			const q = elSearch.value
			if (searchTimer) clearTimeout(searchTimer)
			searchTimer = setTimeout(() => {
				runSearch(q)
			}, 200)
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
		setupTOCBehavior()
		setupNavToggle()
		setupSearch()
		await loadTree()
		await route()
		setupLiveUpdates()
	}

  boot().catch((err) => {
    setStatus('')
    elViewer.innerHTML = `<pre class="error">${esc(err && err.message ? err.message : String(err))}</pre>`
  })
})()
