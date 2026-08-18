// ==UserScript==
// @name         Sub2API 浏览器凭据导出助手
// @namespace    https://github.com/Wei-Shaw/sub2api
// @version      1.1.0
// @description  经用户确认后，将当前 Sub2API 登录 Token 和当前域 Cookie 复制为可导入的 JSON 凭据包。
// @author       Sub2API
// @match        http://*/*
// @match        https://*/*
// @run-at       document-idle
// @noframes
// @grant        GM_addStyle
// @grant        GM_cookie
// @grant        GM_notification
// @grant        GM_registerMenuCommand
// @grant        GM_setClipboard
// ==/UserScript==

(function () {
  'use strict'

  const FORMAT = 'sub2api-browser-credentials'
  const VERSION = 1
  const BUTTON_ID = 'sub2api-credential-exporter-button'
  const PANEL_ID = 'sub2api-credential-exporter-panel'
  const REQUIRED_STORAGE_KEYS = ['auth_token', 'refresh_token']
  let verificationStarted = false
  let verifiedSub2API = false

  GM_addStyle(`
    #${BUTTON_ID} {
      position: fixed;
      right: 20px;
      bottom: 20px;
      z-index: 2147483647;
      display: inline-flex;
      align-items: center;
      gap: 8px;
      min-height: 40px;
      padding: 0 14px;
      border: 1px solid rgba(15, 118, 110, .35);
      border-radius: 6px;
      background: #0f766e;
      color: #fff;
      box-shadow: 0 8px 24px rgba(15, 23, 42, .22);
      font: 600 13px/1 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      letter-spacing: 0;
      cursor: pointer;
    }
    #${BUTTON_ID}:hover { background: #115e59; }
    #${BUTTON_ID}:focus-visible { outline: 2px solid #5eead4; outline-offset: 2px; }
    #${BUTTON_ID}[disabled] { cursor: wait; opacity: .7; }
    #${BUTTON_ID} .sub2api-exporter-dot {
      width: 7px;
      height: 7px;
      border-radius: 50%;
      background: #99f6e4;
      box-shadow: 0 0 0 3px rgba(153, 246, 228, .18);
    }
    #${PANEL_ID} {
      position: fixed;
      inset: 0;
      z-index: 2147483647;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 20px;
      background: rgba(15, 23, 42, .46);
      font: 14px/1.5 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      letter-spacing: 0;
    }
    #${PANEL_ID} * { box-sizing: border-box; letter-spacing: 0; }
    #${PANEL_ID} .sub2api-export-panel {
      width: min(620px, 100%);
      max-height: min(760px, calc(100vh - 40px));
      overflow: auto;
      border: 1px solid #dbe3ea;
      border-radius: 8px;
      background: #fff;
      color: #172033;
      box-shadow: 0 24px 64px rgba(15, 23, 42, .28);
    }
    #${PANEL_ID} .sub2api-export-header {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      gap: 16px;
      padding: 18px 20px 14px;
      border-bottom: 1px solid #e7edf2;
    }
    #${PANEL_ID} h2 { margin: 0; color: #111827; font-size: 17px; font-weight: 700; }
    #${PANEL_ID} .sub2api-export-subtitle { margin: 3px 0 0; color: #64748b; font-size: 12px; }
    #${PANEL_ID} .sub2api-export-close {
      width: 32px;
      height: 32px;
      padding: 0;
      border: 0;
      border-radius: 5px;
      background: transparent;
      color: #64748b;
      font-size: 22px;
      line-height: 1;
      cursor: pointer;
    }
    #${PANEL_ID} .sub2api-export-close:hover { background: #f1f5f9; color: #0f172a; }
    #${PANEL_ID} .sub2api-export-body { padding: 16px 20px 20px; }
    #${PANEL_ID} .sub2api-token-status {
      display: grid;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 8px;
      margin-bottom: 16px;
    }
    #${PANEL_ID} .sub2api-token-status > div {
      min-width: 0;
      padding: 9px 10px;
      border: 1px solid #dbe8e5;
      border-radius: 6px;
      background: #f4fbf9;
      color: #115e59;
      font-size: 12px;
      font-weight: 650;
    }
    #${PANEL_ID} .sub2api-cookie-heading {
      display: flex;
      align-items: baseline;
      justify-content: space-between;
      gap: 12px;
      margin-bottom: 8px;
    }
    #${PANEL_ID} .sub2api-cookie-heading strong { color: #1e293b; font-size: 13px; }
    #${PANEL_ID} .sub2api-cookie-heading span { color: #64748b; font-size: 11px; }
    #${PANEL_ID} .sub2api-cookie-list {
      overflow: hidden;
      border: 1px solid #e2e8f0;
      border-radius: 6px;
      background: #f8fafc;
    }
    #${PANEL_ID} .sub2api-cookie-row {
      display: grid;
      grid-template-columns: minmax(110px, .8fr) minmax(160px, 1.4fr) auto;
      align-items: center;
      gap: 10px;
      min-height: 38px;
      padding: 7px 10px;
      border-bottom: 1px solid #e7edf2;
      color: #475569;
      font-size: 11px;
    }
    #${PANEL_ID} .sub2api-cookie-row:last-child { border-bottom: 0; }
    #${PANEL_ID} .sub2api-cookie-row code {
      overflow: hidden;
      color: #0f172a;
      font: 600 11px/1.4 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    #${PANEL_ID} .sub2api-cookie-path {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    #${PANEL_ID} .sub2api-cookie-flags { color: #0f766e; white-space: nowrap; }
    #${PANEL_ID} .sub2api-cookie-empty { padding: 13px 10px; color: #64748b; font-size: 12px; }
    #${PANEL_ID} .sub2api-export-warning {
      margin: 14px 0 0;
      color: #92400e;
      font-size: 11px;
    }
    #${PANEL_ID} .sub2api-export-json {
      width: 100%;
      min-height: 180px;
      margin-top: 12px;
      padding: 10px;
      resize: vertical;
      border: 1px solid #cbd5e1;
      border-radius: 6px;
      background: #f8fafc;
      color: #1e293b;
      font: 11px/1.5 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    }
    #${PANEL_ID} .sub2api-copy-status { min-height: 20px; margin-top: 10px; color: #475569; font-size: 12px; }
    #${PANEL_ID} .sub2api-copy-status[data-state="success"] { color: #047857; }
    #${PANEL_ID} .sub2api-copy-status[data-state="error"] { color: #b91c1c; }
    #${PANEL_ID} .sub2api-export-actions {
      display: flex;
      flex-wrap: wrap;
      justify-content: flex-end;
      gap: 8px;
      margin-top: 12px;
    }
    #${PANEL_ID} .sub2api-export-actions button {
      min-height: 38px;
      padding: 0 14px;
      border: 1px solid #cbd5e1;
      border-radius: 6px;
      background: #fff;
      color: #334155;
      font: 650 12px/1 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      cursor: pointer;
    }
    #${PANEL_ID} .sub2api-export-actions button:hover { background: #f8fafc; }
    #${PANEL_ID} .sub2api-export-actions .sub2api-copy-button {
      border-color: #0f766e;
      background: #0f766e;
      color: #fff;
    }
    #${PANEL_ID} .sub2api-export-actions .sub2api-copy-button:hover { background: #115e59; }
    #${PANEL_ID} .sub2api-export-actions button[disabled] { cursor: wait; opacity: .65; }
    @media (max-width: 560px) {
      #${PANEL_ID} { align-items: flex-end; padding: 10px; }
      #${PANEL_ID} .sub2api-export-panel { max-height: calc(100vh - 20px); }
      #${PANEL_ID} .sub2api-token-status { grid-template-columns: 1fr; }
      #${PANEL_ID} .sub2api-cookie-row { grid-template-columns: minmax(90px, .8fr) minmax(130px, 1.4fr); }
      #${PANEL_ID} .sub2api-cookie-flags { grid-column: 1 / -1; }
      #${PANEL_ID} .sub2api-export-actions button { flex: 1 1 auto; }
    }
  `)

  function notify(text, isError) {
    GM_notification({
      title: isError ? 'Sub2API 凭据导出失败' : 'Sub2API 凭据导出助手',
      text,
      timeout: isError ? 6000 : 3500,
    })
  }

  function readStorage() {
    const authUserRaw = localStorage.getItem('auth_user')
    let authUser = authUserRaw
    if (authUserRaw) {
      try {
        authUser = JSON.parse(authUserRaw)
      } catch {
        // Older or customized clients may store a non-JSON user value.
      }
    }
    return {
      auth_token: localStorage.getItem('auth_token'),
      refresh_token: localStorage.getItem('refresh_token'),
      token_expires_at: localStorage.getItem('token_expires_at'),
      auth_user: authUser,
    }
  }

  function hasTokenPair() {
    return REQUIRED_STORAGE_KEYS.every(key => Boolean(localStorage.getItem(key)))
  }

  async function verifySub2API() {
    if (verifiedSub2API) return true
    if (verificationStarted || !hasTokenPair()) return false
    verificationStarted = true
    try {
      const response = await fetch('/api/v1/settings/public', {
        method: 'GET',
        credentials: 'include',
        headers: { Accept: 'application/json' },
      })
      const payload = await response.json()
      verifiedSub2API = response.ok && payload && payload.code === 0 && payload.data && typeof payload.data.version === 'string'
    } catch {
      // Some older deployments do not expose public settings. Exact Sub2API
      // storage keys still allow an explicit, user-confirmed manual export.
      verifiedSub2API = hasTokenPair() && Boolean(localStorage.getItem('auth_user'))
    } finally {
      verificationStarted = false
    }
    return verifiedSub2API
  }

  function documentCookieItems() {
    if (!document.cookie.trim()) return []
    return document.cookie.split(';').map(part => {
      const separator = part.indexOf('=')
      const name = (separator >= 0 ? part.slice(0, separator) : part).trim()
      const value = separator >= 0 ? part.slice(separator + 1).trim() : ''
      return {
        name,
        value,
        domain: location.hostname,
        path: '/',
        httpOnly: false,
        secure: location.protocol === 'https:',
        source: 'document.cookie',
      }
    }).filter(cookie => cookie.name)
  }

  function mergeCookieItems(primary, fallback) {
    const merged = new Map()
    const capturedNames = new Set()
    for (const cookie of primary) {
      const domain = String(cookie.domain || location.hostname).replace(/^\./, '').toLowerCase()
      const key = `${domain}\n${cookie.path || '/'}\n${cookie.name}`
      if (!merged.has(key)) merged.set(key, cookie)
      capturedNames.add(cookie.name)
    }
    // document.cookie does not expose Domain or Path. Only add names that the
    // privileged API did not return, otherwise the same cookie may be counted twice.
    for (const cookie of fallback) {
      if (capturedNames.has(cookie.name)) continue
      const key = `${location.hostname.toLowerCase()}\n/\n${cookie.name}`
      if (!merged.has(key)) merged.set(key, cookie)
    }
    return Array.from(merged.values())
  }

  function listCookies() {
    return new Promise(resolve => {
      if (typeof GM_cookie === 'undefined' || typeof GM_cookie.list !== 'function') {
        resolve({
          method: 'document.cookie',
          http_only_support: 'unavailable',
          items: documentCookieItems(),
        })
        return
      }
      GM_cookie.list({ url: location.href }, (cookies, error) => {
        if (error || !Array.isArray(cookies)) {
          resolve({
            method: 'document.cookie',
            http_only_support: 'unavailable',
            error: error ? String(error) : undefined,
            items: documentCookieItems(),
          })
          return
        }
        const visibleCookies = documentCookieItems()
        const gmCookies = cookies.map(cookie => ({
          name: cookie.name,
          value: cookie.value,
          domain: cookie.domain,
          path: cookie.path,
          expirationDate: cookie.expirationDate,
          hostOnly: cookie.hostOnly,
          httpOnly: cookie.httpOnly,
          sameSite: cookie.sameSite,
          secure: cookie.secure,
          session: cookie.session,
          partitionKey: cookie.partitionKey,
          source: 'GM_cookie.list',
        }))
        resolve({
          method: visibleCookies.length ? 'GM_cookie.list+document.cookie' : 'GM_cookie.list',
          // Tampermonkey documents HttpOnly access as Beta-only. Individual
          // cookie flags below are the authoritative evidence for this capture.
          http_only_support: 'tampermonkey-beta-only',
          items: mergeCookieItems(gmCookies, visibleCookies),
        })
      })
    })
  }

  async function copyText(text, manualTextarea) {
    if (window.isSecureContext && navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
      try {
        await navigator.clipboard.writeText(text)
        return 'browser'
      } catch {
        // Continue with the userscript and legacy fallbacks.
      }
    }
    if (typeof GM_setClipboard === 'function') {
      try {
        // The string form is supported across more Tampermonkey versions than
        // the metadata-object overload.
        GM_setClipboard(text, 'text')
        return 'tampermonkey'
      } catch {
        // Continue with the selectable JSON fallback.
      }
    }
    if (manualTextarea) {
      manualTextarea.hidden = false
      manualTextarea.focus()
      manualTextarea.select()
      if (document.execCommand('copy')) return 'legacy'
    }
    throw new Error('浏览器拒绝写入剪贴板，请展开完整 JSON 后手动复制。')
  }

  function showExportPanel(bundleText, storage, cookieCapture) {
    document.getElementById(PANEL_ID)?.remove()

    const overlay = document.createElement('div')
    overlay.id = PANEL_ID
    overlay.setAttribute('role', 'dialog')
    overlay.setAttribute('aria-modal', 'true')
    overlay.setAttribute('aria-labelledby', `${PANEL_ID}-title`)
    overlay.innerHTML = `
      <section class="sub2api-export-panel">
        <header class="sub2api-export-header">
          <div>
            <h2 id="${PANEL_ID}-title">Sub2API 凭据已生成</h2>
            <p class="sub2api-export-subtitle"></p>
          </div>
          <button type="button" class="sub2api-export-close" aria-label="关闭">&times;</button>
        </header>
        <div class="sub2api-export-body">
          <div class="sub2api-token-status">
            <div>Access Token 已包含</div>
            <div>Refresh Token 已包含</div>
            <div class="sub2api-cookie-count"></div>
          </div>
          <div class="sub2api-cookie-heading">
            <strong>Cookie 信息</strong>
            <span>仅显示名称和作用域，不显示值</span>
          </div>
          <div class="sub2api-cookie-list"></div>
          <p class="sub2api-export-warning">完整凭据包含敏感 Token。请只复制到你自己的 Sub2API 上游管理页面。</p>
          <textarea class="sub2api-export-json" readonly hidden aria-label="完整 JSON 凭据包"></textarea>
          <div class="sub2api-copy-status" role="status" aria-live="polite">尚未复制，请点击下方按钮。</div>
          <div class="sub2api-export-actions">
            <button type="button" class="sub2api-toggle-json">显示完整 JSON</button>
            <button type="button" class="sub2api-copy-button">复制完整凭据包</button>
          </div>
        </div>
      </section>
    `

    const closeButton = overlay.querySelector('.sub2api-export-close')
    const subtitle = overlay.querySelector('.sub2api-export-subtitle')
    const cookieCount = overlay.querySelector('.sub2api-cookie-count')
    const cookieList = overlay.querySelector('.sub2api-cookie-list')
    const textarea = overlay.querySelector('.sub2api-export-json')
    const status = overlay.querySelector('.sub2api-copy-status')
    const toggleButton = overlay.querySelector('.sub2api-toggle-json')
    const copyButton = overlay.querySelector('.sub2api-copy-button')

    subtitle.textContent = `${location.origin} · ${storage.auth_user?.email || '当前登录账号'}`
    cookieCount.textContent = `Cookie ${cookieCapture.items.length} 个`
    textarea.value = bundleText

    if (cookieCapture.items.length === 0) {
      const empty = document.createElement('div')
      empty.className = 'sub2api-cookie-empty'
      empty.textContent = '当前站点没有可读取 Cookie；Token 对仍可正常导入。'
      cookieList.appendChild(empty)
    } else {
      for (const cookie of cookieCapture.items.slice(0, 5)) {
        const row = document.createElement('div')
        row.className = 'sub2api-cookie-row'
        const name = document.createElement('code')
        name.textContent = cookie.name
        const path = document.createElement('span')
        path.className = 'sub2api-cookie-path'
        path.textContent = `${cookie.domain || location.hostname}${cookie.path || '/'}`
        const flags = document.createElement('span')
        flags.className = 'sub2api-cookie-flags'
        flags.textContent = [cookie.httpOnly ? 'HttpOnly' : '', cookie.secure ? 'Secure' : ''].filter(Boolean).join(' · ') || '普通 Cookie'
        row.append(name, path, flags)
        cookieList.appendChild(row)
      }
      if (cookieCapture.items.length > 5) {
        const more = document.createElement('div')
        more.className = 'sub2api-cookie-empty'
        more.textContent = `另有 ${cookieCapture.items.length - 5} 个 Cookie 已包含在凭据包中。`
        cookieList.appendChild(more)
      }
    }

    closeButton.addEventListener('click', () => overlay.remove())
    overlay.addEventListener('click', event => {
      if (event.target === overlay) overlay.remove()
    })
    toggleButton.addEventListener('click', () => {
      textarea.hidden = !textarea.hidden
      toggleButton.textContent = textarea.hidden ? '显示完整 JSON' : '隐藏完整 JSON'
      if (!textarea.hidden) {
        textarea.focus()
        textarea.select()
      }
    })
    copyButton.addEventListener('click', async () => {
      copyButton.disabled = true
      status.dataset.state = ''
      status.textContent = '正在写入剪贴板...'
      try {
        const method = await copyText(bundleText, textarea)
        status.dataset.state = 'success'
        status.textContent = method === 'browser'
          ? '复制成功。现在可以回到上游管理页面粘贴。'
          : '已调用复制功能。请立即粘贴验证；如果没有内容，请展开 JSON 手动复制。'
      } catch (error) {
        status.dataset.state = 'error'
        status.textContent = error instanceof Error ? error.message : '复制失败，请展开完整 JSON 后手动复制。'
      } finally {
        copyButton.disabled = false
      }
    })

    document.body.appendChild(overlay)
    copyButton.focus()
  }

  async function exportCredentials(button) {
    const storage = readStorage()
    if (!storage.auth_token || !storage.refresh_token) {
      notify('没有检测到完整的 Access Token 和 Refresh Token，请先完成登录。', true)
      return
    }
    const accepted = window.confirm(
      '即将生成当前站点的登录 Token 和可读取 Cookie，并在页面面板中提供复制按钮。凭据可控制当前账号，请只粘贴到你自己的 Sub2API 上游管理页面。\n\n建议从专用或临时浏览器会话导出，导入后关闭该会话，避免浏览器与管理端同时轮换同一枚 Refresh Token。是否继续？'
    )
    if (!accepted) return

    if (button) button.disabled = true
    try {
      const cookieCapture = await listCookies()
      const credentialBundle = {
        format: FORMAT,
        version: VERSION,
        source: {
          origin: location.origin,
          captured_at: new Date().toISOString(),
          user_agent: navigator.userAgent,
        },
        local_storage: storage,
        cookie_capture: {
          ...cookieCapture,
          document_cookie: document.cookie,
          cookie_header: cookieCapture.items.map(cookie => `${cookie.name}=${cookie.value}`).join('; '),
        },
      }
      const bundleText = JSON.stringify(credentialBundle, null, 2)
      showExportPanel(bundleText, storage, cookieCapture)
      notify('凭据已生成。请在页面面板查看 Cookie 信息，并点击“复制完整凭据包”。')
    } catch (error) {
      notify(error instanceof Error ? error.message : '无法生成凭据包。', true)
    } finally {
      if (button) button.disabled = false
    }
  }

  async function mountButtonWhenReady() {
    if (document.getElementById(BUTTON_ID) || !hasTokenPair()) return
    if (!await verifySub2API()) return
    const button = document.createElement('button')
    button.id = BUTTON_ID
    button.type = 'button'
    button.title = '导出当前 Sub2API 登录 Token 与可读取 Cookie'
    button.innerHTML = '<span class="sub2api-exporter-dot" aria-hidden="true"></span><span>导出 Sub2API 凭据</span>'
    button.addEventListener('click', () => exportCredentials(button))
    document.body.appendChild(button)
  }

  GM_registerMenuCommand('复制当前 Sub2API 登录凭据', () => exportCredentials(null))
  void mountButtonWhenReady()
  window.setInterval(() => { void mountButtonWhenReady() }, 1500)
})()
