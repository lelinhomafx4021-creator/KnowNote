/** 使用真实前端 API/工作台状态，经 Vite 代理访问运行中的后端；仅生成可删除的测试资料。 */
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'
import { performance } from 'node:perf_hooks'

// 动态打包模块的堆栈含整段 data URL，失败时只输出必要的断言信息。
process.on('uncaughtException', (error) => { console.error(`端到端验收失败：${error.message}`); process.exitCode = 1 })

const base = process.env.E2E_BASE_URL || 'http://localhost:5173'
const origin = new URL(base).origin
const root = fileURLToPath(new URL('../', import.meta.url))
const { build } = await import(createRequire(import.meta.resolve('vite')).resolve('esbuild'))
const memory = new Map()
globalThis.localStorage = { getItem: (key) => memory.get(key) ?? null, setItem: (key, value) => memory.set(key, value), removeItem: (key) => memory.delete(key) }
globalThis.document = { documentElement: { classList: { contains: () => false } } }
globalThis.window = { location: { origin } }
const realFetch = globalThis.fetch
globalThis.fetch = (path, init = {}) => realFetch(new URL(path, base), { ...init, headers: { ...Object.fromEntries(new Headers(init.headers)), Origin: origin }, signal: init.signal ?? AbortSignal.timeout(90000) })
const bundle = await build({
  stdin: { contents: `export { authApi } from './api/auth'; export { notebooksApi } from './api/notebooks'; export { sourcesApi } from './api/sources'; export { notesApi } from './api/notes'; export { chatApi } from './api/chat'; export { studioApi } from './api/studio'; export { useWorkspace } from './store/useWorkspaceStore'; export { useAuthStore } from './store/useAuthStore';`, resolveDir: root + 'src' },
  bundle: true, write: false, format: 'esm', platform: 'node', alias: { '@': root + 'src' },
  define: { 'import.meta.env.VITE_USE_MOCK': '"false"', 'import.meta.env.VITE_API_BASE_URL': '"/api"' },
})
const { authApi, notebooksApi, sourcesApi, notesApi, chatApi, studioApi, useWorkspace, useAuthStore } = await import('data:text/javascript;base64,' + Buffer.from(bundle.outputFiles[0].text).toString('base64'))

// 直接构造带文字的 PDF，避免测试依赖外部文档或系统字体。
function pdfFixture() {
  const stream = 'BT /F1 12 Tf 72 720 Td (KnowNote uses PostgreSQL and pgvector for vector search and citations.) Tj ET'
  const objects = ['<< /Type /Catalog /Pages 2 0 R >>', '<< /Type /Pages /Kids [3 0 R] /Count 1 >>', '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>', '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>', `<< /Length ${stream.length} >>\nstream\n${stream}\nendstream`]
  let pdf = '%PDF-1.4\n'
  const offsets = [0]
  objects.forEach((object, i) => { offsets.push(Buffer.byteLength(pdf)); pdf += `${i + 1} 0 obj\n${object}\nendobj\n` })
  const xref = Buffer.byteLength(pdf)
  pdf += `xref\n0 ${offsets.length}\n0000000000 65535 f \n${offsets.slice(1).map((offset) => `${String(offset).padStart(10, '0')} 00000 n \n`).join('')}trailer\n<< /Size ${offsets.length} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`
  return new File([pdf], '联调资料.pdf', { type: 'application/pdf' })
}

const metrics = { ingestion: [], streamingUpdates: 0 }
let notebook
let token
const input = { email: `t6-${crypto.randomUUID()}@example.com`, password: 'knownote-t6-test', nickname: '收尾联调' }
async function timedSource(kind, create) {
  const start = performance.now()
  const source = await create()
  assert.equal(source.status, 'ready', `${kind} 处理失败：${source.error}`)
  assert.ok(source.content && source.summary && source.topics.length >= 3)
  assert.ok(source.topics.every((topic) => !/^\d+$/.test(topic) && topic !== '合大'))
  metrics.ingestion.push({ kind, milliseconds: Math.round(performance.now() - start), characters: source.wordCount })
  return source
}

try {
  const health = await (await fetch('/api/health')).json()
  assert.equal(health.data.db, 'ok')
  const viteModule = await (await fetch('/src/api/client.ts')).text()
  assert.match(viteModule, /"VITE_USE_MOCK"\s*:\s*"false"/)
  const registered = await authApi.register(input)
  useAuthStore.getState().accept(registered)
  useAuthStore.getState().logout()
  assert.equal(memory.get('kn-token'), undefined)
  const logged = await authApi.login({ email: input.email, password: input.password })
  token = logged.token
  useAuthStore.getState().accept(logged)
  assert.equal((await authApi.me(token)).id, registered.user.id)
  console.log('通过：真实模式、数据库健康、注册、退出后重新登录、当前用户')
  notebook = await notebooksApi.create('T6 端到端验收')
  await useWorkspace.getState().load(notebook.id)
  assert.deepEqual(useWorkspace.getState().moduleErrors, [])

  const pdf = await timedSource('PDF', async () => {
    const form = new FormData()
    form.append('file', pdfFixture())
    const response = await fetch(`/api/notebooks/${notebook.id}/sources/upload`, { method: 'POST', headers: { Authorization: `Bearer ${token}` }, body: form })
    assert.equal(response.status, 200)
    const initial = (await response.json()).data
    assert.equal(initial.status, 'processing')
    return sourcesApi.poll(initial, undefined, AbortSignal.timeout(60000))
  })
  const web = await timedSource('网页', () => sourcesApi.add(notebook.id, { type: 'web', title: '示例网页', url: 'https://example.com' }, undefined, AbortSignal.timeout(60000)))
  const text = await timedSource('文本', () => sourcesApi.add(notebook.id, { type: 'text', title: '毕业设计架构说明', text: '2026年完成知页毕业设计。知页融合大模型与向量检索，采用 PostgreSQL 和 pgvector 管理个人资料。pgvector 使结构化数据与向量保存在同一数据库中，减少部署组件，事务能保证来源和分块的一致性。HNSW 索引加速余弦相似度检索。文本约500字切分，保留80字重叠。引用标注让回答能回溯原文。JWT 用于用户认证，每次查询按用户隔离。'.repeat(12) }, undefined, AbortSignal.timeout(60000)))
  assert.match(pdf.content, /KnowNote/)
  assert.match(web.content, /documentation/i)
  await useWorkspace.getState().load(notebook.id)
  assert.equal(useWorkspace.getState().selectedIds.length, 3)
  const guide = await notebooksApi.guide(notebook.id)
  assert.ok(guide.summary && guide.suggestedQuestions.length === 3)
  console.log('通过：PDF 上传、真实公网网页、中文文本、三来源 ready、摘要/主题/概要')

  const start = performance.now()
  let firstContent = false
  let firstCitations = false
  const unsubscribe = useWorkspace.subscribe((state, previous) => {
    const answer = state.messages.at(-1)
    if (answer?.role !== 'assistant') return
    if (!firstCitations && answer.citations?.length) { firstCitations = true; metrics.retrievalChainMs = Math.round(performance.now() - start) }
    if (answer.content && answer.content !== previous.messages.at(-1)?.content) {
      metrics.streamingUpdates++
      if (!firstContent) { firstContent = true; metrics.firstDeltaMs = Math.round(performance.now() - start) }
    }
  })
  await useWorkspace.getState().ask('为什么毕设用 pgvector？')
  unsubscribe()
  metrics.chatTotalMs = Math.round(performance.now() - start)
  const answer = useWorkspace.getState().messages.at(-1)
  assert.ok(!useWorkspace.getState().streaming && metrics.streamingUpdates > 1)
  assert.ok(answer.citations.length && /\[\d+\]/.test(answer.content))
  const history = await chatApi.history(notebook.id)
  const saved = history.at(-1)
  assert.equal(saved.id, answer.id)
  assert.equal(saved.content, answer.content)
  assert.ok(saved.citations.length)
  const markers = [...answer.content.matchAll(/\[(\d+)\]/g)].map((match) => Number(match[1]))
  for (const index of markers) {
    const citation = answer.citations.find((item) => item.index === index)
    assert.ok(citation && citation.quote)
    const source = [pdf, web, text].find((item) => item.id === citation.sourceId)
    assert.equal(citation.sourceTitle, source.title)
    assert.ok(source.content.replace(/\s+/g, ' ').includes(citation.quote))
    // 与 CitationChip 的点击处理一致，检查来源选择和展示片段。
    useWorkspace.getState().openSource(citation.sourceId, citation.quote)
    assert.equal(useWorkspace.getState().openSourceId, source.id)
    assert.equal(useWorkspace.getState().highlight, citation.quote)
  }
  for (const value of ['up', 'down', null]) {
    await chatApi.feedback(answer.id, value)
    assert.equal((await chatApi.history(notebook.id)).at(-1).feedback ?? null, value)
  }
  const note = await useWorkspace.getState().createNote({ title: 'pgvector 选型依据', content: answer.content.replace(/\[(\d+)\]/g, ''), origin: 'chat' }, false)
  assert.ok(note && (await notesApi.list(notebook.id)).some((item) => item.id === note.id && item.origin === 'chat'))
  await notesApi.update(note.id, { title: '已编辑的选型依据' })
  assert.equal((await notesApi.list(notebook.id))[0].title, '已编辑的选型依据')
  console.log('通过：带引用 SSE、逐段状态更新、消息回读、引用数据/跳转状态、反馈清除、保存/编辑笔记')

  for (const kind of ['study-guide', 'briefing', 'faq', 'timeline', 'mindmap']) {
    await useWorkspace.getState().generate(kind)
    const artifact = useWorkspace.getState().artifacts.find((item) => item.kind === kind)
    assert.ok(artifact, `${kind} 未成功生成`)
    assert.ok(kind === 'mindmap' ? artifact.mindmap?.children?.length : artifact.content?.length)
  }
  assert.equal((await studioApi.list(notebook.id)).length, 5)
  console.log('通过：五种工作室产物持久化')

  await sourcesApi.remove(pdf.id)
  assert.equal((await sourcesApi.list(notebook.id)).length, 2)
  assert.equal((await notebooksApi.get(notebook.id)).sourceCount, 2)
  await notesApi.remove(note.id)
  assert.equal((await notesApi.list(notebook.id)).length, 0)
  await notebooksApi.remove(notebook.id)
  notebook = undefined
  assert.equal((await notebooksApi.list()).length, 0)
  useAuthStore.getState().logout()
  assert.equal(useAuthStore.getState().token, null)
  assert.equal(memory.get('kn-token'), undefined)
  assert.equal(useWorkspace.getState().notebookId, null)
  assert.equal(useWorkspace.getState().messages.length, 0)
  assert.equal((await fetch('/api/me')).status, 401)
  console.log('通过：删除来源/笔记/笔记本、级联可见结果、退出清理前端会话、无 token 返回 401')
  console.log('实测指标：' + JSON.stringify(metrics))
  console.log('端到端验收全部通过；测试用户保留（无用户删除接口），业务资料已清理。')
} finally {
  if (notebook && token) {
    useAuthStore.setState({ token })
    await notebooksApi.remove(notebook.id).catch(() => console.error('测试笔记本清理失败，请按 T6 标题手动清理'))
  }
  useAuthStore.getState().logout()
}
