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
	const elThemeToggle = document.getElementById('themeToggle')

	let tree = null
	let currentPath = ''
	let currentMTime = 0
	let scrollSpyDisconnect = null
	let searchTimer = null
	let lastQuery = ''
	let currentSearchQuery = ''
	let currentHighlightIdx = -1
	const openDirPaths = new Set()
	const closedDirPaths = new Set()
	let navCollapsed = false
	let elHighlightNav = null
	let elHighlightCount = null

	function syncNavToggle() {
		if (!elNavToggle) return
		elNavToggle.textContent = navCollapsed ? 'Expand' : 'Collapse'
	}

	function setupThemeToggle() {
		if (!elThemeToggle) return
		let theme = 'light'
		try {
			theme = localStorage.getItem('repobook.theme') || 'light'
		} catch (_) {}
		applyTheme(theme)
		elThemeToggle.addEventListener('click', () => {
			const next = document.body.getAttribute('data-theme') === 'dark' ? 'light' : 'dark'
			applyTheme(next)
			try {
				localStorage.setItem('repobook.theme', next)
			} catch (_) {}
		})
	}

	function applyTheme(theme) {
		if (theme === 'dark') {
			document.body.setAttribute('data-theme', 'dark')
			elThemeToggle.textContent = '\u2600'
			elThemeToggle.setAttribute('aria-label', 'Switch to light mode')
		} else {
			document.body.removeAttribute('data-theme')
			elThemeToggle.textContent = '\u263E'
			elThemeToggle.setAttribute('aria-label', 'Switch to dark mode')
		}
	}

	function addCopyButtons() {
		elViewer.querySelectorAll('pre').forEach((pre) => {
			if (pre.querySelector('.copy-code-btn')) return
			const wrapper = document.createElement('div')
			wrapper.className = 'code-block-wrapper'
			pre.parentNode.insertBefore(wrapper, pre)
			wrapper.appendChild(pre)
			const btn = document.createElement('button')
			btn.className = 'copy-code-btn'
			btn.textContent = 'Copy'
			btn.addEventListener('click', () => {
				const code = pre.querySelector('code')
				const text = code ? code.textContent : pre.textContent
				navigator.clipboard.writeText(text).then(() => {
					btn.textContent = 'Copied!'
					btn.classList.add('copied')
					setTimeout(() => {
						btn.textContent = 'Copy'
						btn.classList.remove('copied')
					}, 2000)
				})
			})
			wrapper.appendChild(btn)
		})
	}

	function highlightSearchTerms(query) {
		clearHighlights()
		if (!query) return
		currentSearchQuery = query
		const terms = query.toLowerCase().split(/\s+/).filter(Boolean)
		if (!terms.length) return

		let count = 0
		const walker = document.createTreeWalker(elViewer, NodeFilter.SHOW_TEXT, {
			acceptNode(node) {
				if (!node.parentElement) return NodeFilter.FILTER_REJECT
				const tag = node.parentElement.tagName
				if (tag === 'SCRIPT' || tag === 'STYLE') return NodeFilter.FILTER_REJECT
				if (node.parentElement.classList.contains('search-highlight')) return NodeFilter.FILTER_REJECT
				return NodeFilter.FILTER_ACCEPT
			}
		})
		const textNodes = []
		while (walker.nextNode()) textNodes.push(walker.currentNode)
		for (const textNode of textNodes) {
			const text = textNode.textContent
			const lower = text.toLowerCase()
			const fragments = []
			for (const term of terms) {
				let idx = lower.indexOf(term)
				while (idx !== -1) {
					fragments.push({ start: idx, end: idx + term.length })
					idx = lower.indexOf(term, idx + term.length)
				}
			}
			if (!fragments.length) continue
			fragments.sort((a, b) => a.start - b.start)
			const merged = [fragments[0]]
			for (let i = 1; i < fragments.length; i++) {
				const last = merged[merged.length - 1]
				if (fragments[i].start <= last.end) {
					last.end = Math.max(last.end, fragments[i].end)
				} else {
					merged.push(fragments[i])
				}
			}
			const span = document.createElement('span')
			let pos = 0
			for (const frag of merged) {
				if (frag.start > pos) {
					span.appendChild(document.createTextNode(text.slice(pos, frag.start)))
				}
				const mark = document.createElement('mark')
				mark.className = 'search-highlight'
				mark.setAttribute('data-highlight-idx', count++)
				mark.textContent = text.slice(frag.start, frag.end)
				span.appendChild(mark)
				pos = frag.end
			}
			if (pos < text.length) {
				span.appendChild(document.createTextNode(text.slice(pos)))
			}
			textNode.parentNode.replaceChild(span, textNode)
		}
		currentHighlightIdx = -1
		updateHighlightNav()
	}

	function getHighlightCount() {
		return elViewer.querySelectorAll('mark.search-highlight').length
	}

	function jumpToHighlight(idx) {
		const marks = elViewer.querySelectorAll('mark.search-highlight')
		if (!marks.length) return
		// Wrap around
		if (idx < 0) idx = marks.length - 1
		if (idx >= marks.length) idx = 0
		currentHighlightIdx = idx
		// Remove current class from all, add to target
		marks.forEach(m => m.classList.remove('current'))
		const target = marks[idx]
		target.classList.add('current')
		target.scrollIntoView({ block: 'center', behavior: 'smooth' })
		updateHighlightNav()
	}

	function jumpToNextHighlight() {
		jumpToHighlight(currentHighlightIdx + 1)
	}

	function jumpToPrevHighlight() {
		jumpToHighlight(currentHighlightIdx - 1)
	}

	function updateHighlightNav() {
		const count = getHighlightCount()
		if (!elHighlightNav) return
		if (count === 0 || !currentSearchQuery) {
			elHighlightNav.hidden = true
			return
		}
		elHighlightNav.hidden = false
		elHighlightCount.textContent = `${currentHighlightIdx + 1} of ${count}`
	}

	function clearHighlights() {
		currentSearchQuery = ''
		currentHighlightIdx = -1
		elViewer.querySelectorAll('mark.search-highlight').forEach((mark) => {
			const parent = mark.parentNode
			parent.replaceChild(document.createTextNode(mark.textContent), mark)
		})
		// Remove wrapper spans that only contain text
		elViewer.querySelectorAll('span').forEach((span) => {
			if (!span.className && span.childNodes.length > 0) {
				let allText = true
				for (const child of span.childNodes) {
					if (child.nodeType !== 3) { allText = false; break }
				}
				if (allText) {
					span.parentNode.replaceChild(document.createTextNode(span.textContent), span)
				}
			}
		})
		updateHighlightNav()
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

  // Load a script once and optionally verify it's available via `verifyFn`.
  // Returns true if the script is loaded and verifyFn (if provided) returns truthy.
  async function loadScriptOnce(src, verifyFn) {
    // If already present, wait briefly for it to initialize.
    if (document.querySelector('script[data-src="' + src + '"]')) {
      if (typeof verifyFn !== 'function') return true
      for (let i = 0; i < 50; i++) {
        if (verifyFn()) return true
        await new Promise((r) => setTimeout(r, 100))
      }
      return !!verifyFn()
    }

    return new Promise((resolve) => {
      const s = document.createElement('script')
      s.async = true
      s.setAttribute('data-src', src)
      s.src = src
      s.onload = () => {
        try {
          resolve(typeof verifyFn === 'function' ? !!verifyFn() : true)
        } catch (e) {
          resolve(false)
        }
      }
      s.onerror = () => resolve(false)
      document.head.appendChild(s)
    })
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

		const hasReadme = !!(node.children || []).find((c) => {
			return c && c.type === 'file' && typeof c.name === 'string' && c.name.toLowerCase() === 'readme.md'
		})

		const active = isPathInDir(currentPath, node.path) ? ' is-active' : ''
		const shouldOpen = !closedDirPaths.has(node.path) && (!navCollapsed || !node.path || active || openDirPaths.has(node.path))
		const openAttr = shouldOpen ? ' open' : ''
		const dirId = 'dir-' + btoa(unescape(encodeURIComponent(node.path || 'root'))).replace(/=+$/g, '')
		const children = (node.children || []).map(renderTreeNode).join('')
		const dirPath = node.path || ''
		const readmeAttr = hasReadme ? '1' : '0'
		const readmeClass = hasReadme ? '' : ' no-readme'
		const titleAttr = hasReadme ? '' : ' title="No README.md in this folder"'
		const label = hasReadme
			? `<a class="nav-dir-link" href="/file/${encodeURI(dirPath)}">${esc(node.name || 'root')}</a>`
			: `<span class="nav-dir-link is-disabled" aria-disabled="true"${titleAttr}>${esc(node.name || 'root')}</span>`
		return (
			`<details class="nav-dir${active}${readmeClass}" id="${dirId}" data-path="${esc(node.path || '')}" data-has-readme="${readmeAttr}"${openAttr}>` +
				`<summary class="nav-dir-title">` +
					`<button class="nav-dir-toggle" type="button" aria-label="Toggle folder"></button>` +
					label +
				`</summary>` +
				`<div class="nav-dir-children">${children}</div>` +
			`</details>`
		)
	}

	function renderTree() {
		if (!tree) return
		elNav.innerHTML = renderTreeNode(tree)

		// Folder title behavior:
		// - clicking the text navigates to the folder README.md (server resolves folder -> README.md)
		// - expanding/collapsing only happens via the arrow button
		elNav.querySelectorAll('details.nav-dir').forEach((d) => {
			const summary = d.querySelector('summary.nav-dir-title')
			if (!summary) return
			summary.addEventListener('click', (e) => {
				// Prevent the native <details>/<summary> toggle.
				e.preventDefault()
				e.stopPropagation()

				const toggleBtn = e.target && e.target.closest ? e.target.closest('button.nav-dir-toggle') : null
				if (toggleBtn) {
					d.open = !d.open
					return
				}
				const link = e.target && e.target.closest ? e.target.closest('.nav-dir-link') : null
				if (!link) return
				if (d.getAttribute('data-has-readme') !== '1') return
				const p = d.getAttribute('data-path') || ''
				navigate(`/file/${encodeURI(p)}`, false)
			})
		})

		// Persist manual open/close.
		elNav.querySelectorAll('details.nav-dir').forEach((d) => {
			d.addEventListener('toggle', () => {
				const p = d.getAttribute('data-path') || ''
				if (!p) return
				if (d.open) {
					openDirPaths.add(p)
					closedDirPaths.delete(p)
				} else {
					openDirPaths.delete(p)
					closedDirPaths.add(p)
				}
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
			else closedDirPaths.clear()
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
		currentSearchQuery = q
		if (!q) {
			setSearchMeta('')
			showResults(false)
			clearHighlights()
			return
		}
		setSearchMeta('Searching…')
		showResults(true)
		try {
			const data = await fetchJSON(`/api/search?q=${encodeURIComponent(q)}`)
			if (lastQuery !== q) return
			renderResults(data)
			setSearchMeta(`${data.results.length}${data.truncated ? '+' : ''} results`)
			highlightSearchTerms(q)
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

  // Render any inserted Mermaid blocks if the runtime is available.
  function renderMermaidElements() {
    try {
      const els = elViewer.querySelectorAll('.mermaid')
      if (!els || !els.length) return
      if (window.mermaid) {
        if (typeof window.mermaid.init === 'function') {
          window.mermaid.init(undefined, els)
          return
        }
        if (typeof window.mermaid.contentLoaded === 'function') {
          window.mermaid.contentLoaded()
          return
        }
        if (typeof window.mermaid.run === 'function') {
          window.mermaid.run()
          return
        }
      }
    } catch (e) {
      console.warn('mermaid render failed', e)
    }
  }

	async function loadDoc(relPath, opts) {
    const anchor = (opts && opts.anchor) || ''
    const fromSearch = opts && opts.fromSearch
    setStatus('Loading…')
    const data = await fetchJSON(`/api/render?path=${encodeURIComponent(relPath)}`)
    currentPath = data.path
    currentMTime = data.mtime || 0
    document.title = `repobook \u2022 ${data.title || data.path}`
    setCrumb(data.path)

    elViewer.innerHTML = `<article class="markdown-body">${data.html}</article>`

    // Add copy buttons to code blocks
    addCopyButtons()

    // Render Mermaid diagrams if the runtime is available. This supports
    // different mermaid API variants across versions and is tolerant to
    // failures (non-fatal).
    renderMermaidElements()

    renderTOC(data.toc || [])
		renderTree()
		setupScrollSpy()
		setStatus('')

    // Apply search highlights if navigating from search results
    if (fromSearch && currentSearchQuery) {
      highlightSearchTerms(currentSearchQuery)
      // Jump to first highlight
      if (getHighlightCount() > 0) {
        jumpToHighlight(0)
      }
      // Do NOT close the results list - user may want to visit other matches
    }

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

  function navigate(urlPath, replace, fromSearch) {
    if (replace) {
      history.replaceState({}, '', urlPath)
    } else {
      history.pushState({}, '', urlPath)
    }
    route(fromSearch)
  }

  async function route(fromSearch) {
    const p = getRoutePath()
    if (!p) {
      await ensureHome()
      return
    }
    await loadDoc(p, { fromSearch: fromSearch })
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
          // If clicking from search results, keep query for highlighting
          const fromSearch = elResults && !elResults.hidden && currentSearchQuery
          navigate(u.pathname + u.hash, false, fromSearch)
	}
      } catch (_) {
        // ignore
      }
    })

    window.addEventListener('popstate', () => {
      clearHighlights()
      if (elSearch) elSearch.value = ''
      currentSearchQuery = ''
      route()
    })
  }

	function setupSearch() {
		if (!elSearch) return

		// Wrap input in a container for positioning the clear button
		const inputWrap = document.createElement('div')
		inputWrap.className = 'search-input-wrap'
		elSearch.parentNode.insertBefore(inputWrap, elSearch)
		inputWrap.appendChild(elSearch)

		// Clear button
		const clearBtn = document.createElement('button')
		clearBtn.type = 'button'
		clearBtn.className = 'search-clear'
		clearBtn.innerHTML = '&times;'
		clearBtn.setAttribute('aria-label', 'Clear search')
		clearBtn.hidden = true
		inputWrap.appendChild(clearBtn)

		function syncClearBtn() {
			clearBtn.hidden = !elSearch.value
		}

		elSearch.addEventListener('input', () => {
			syncClearBtn()
			const q = elSearch.value
			if (searchTimer) clearTimeout(searchTimer)
			searchTimer = setTimeout(() => {
				runSearch(q)
			}, 200)
		})

		clearBtn.addEventListener('click', () => {
			elSearch.value = ''
			syncClearBtn()
			runSearch('')
			clearHighlights()
			elSearch.focus()
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
    setupThemeToggle()
    // Highlight nav - must be assigned here (after DOM is ready, after let declarations)
    elHighlightNav = document.getElementById('highlightNav')
    elHighlightCount = document.getElementById('highlightCount')
    if (elHighlightNav) {
      elHighlightNav.addEventListener('click', (e) => {
        const btn = e.target.closest('[data-dir]')
        if (!btn) return
        if (btn.getAttribute('data-dir') === 'prev') jumpToPrevHighlight()
        else jumpToNextHighlight()
      })
    }
    // Load mermaid runtime lazily. Prefer a vendored local copy embedded into
    // the app (served under /app/vendor/mermaid.min.js) so offline/CI runs can
    // work without network access. Fall back to CDN if a local file is missing.
    if (typeof window.mermaid === 'undefined') {
      const tryLocal = async () => {
        try {
          return await loadScriptOnce('/app/vendor/mermaid.min.js', () => typeof window.mermaid !== 'undefined')
        } catch (_) {
          return false
        }
      }
      const okLocal = await tryLocal()
      if (!okLocal) {
        // Fallback CDN (unpkg pinned version). This is convenient during
        // development but CI/air-gapped environments should vendor the file.
        try {
          await loadScriptOnce('https://unpkg.com/mermaid@10.4.0/dist/mermaid.min.js', () => typeof window.mermaid !== 'undefined')
        } catch (_) {
          // Non-fatal: pages will still show the mermaid source block.
        }
      }
      if (window.mermaid && window.mermaid.initialize) {
        window.mermaid.initialize({ startOnLoad: false })
      }
    }
    await loadTree()
    await route()
    setupLiveUpdates()
  }

  boot().catch((err) => {
    setStatus('')
    elViewer.innerHTML = `<pre class="error">${esc(err && err.message ? err.message : String(err))}</pre>`
  })
})()
