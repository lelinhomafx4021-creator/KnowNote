/** 对传输分包、取消和并发状态做确定性验证，无需数据库或浏览器。 */
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
// 复用 Vite 固定依赖的编译器，不新增测试框架或全局依赖。
const { build } = await import(createRequire(import.meta.resolve('vite')).resolve('esbuild'))
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('../', import.meta.url))
const result = await build({
  stdin: { contents: `export * from './api/client'; export { sourcesApi, sourceFileUrl } from './api/sources'; export { studioApi } from './api/studio'; export { useWorkspace } from './store/useWorkspaceStore'; export { useAuthStore } from './store/useAuthStore';`, resolveDir: root + 'src' },
  bundle: true, write: false, format: 'esm', platform: 'node', alias: { '@': root + 'src' },
  define: { 'import.meta.env.VITE_USE_MOCK': '"false"', 'import.meta.env.VITE_API_BASE_URL': '"/api"' },
  plugins: [{ name: '测试提示收集', setup(build) { build.onResolve({ filter: /useUIStore$/ }, () => ({ path: 'ui', namespace: 'test' })); build.onLoad({ filter: /.*/, namespace: 'test' }, () => ({ contents: 'export const toast = (...args) => globalThis.notices.push(args); export const useUIStore = { getState: () => ({ setSettingsOpen() {} }) }' })) } }],
})
globalThis.notices = []
const api = await import('data:text/javascript;base64,' + Buffer.from(result.outputFiles[0].text).toString('base64'))
const encoder = new TextEncoder()
const envelope = (data, status = 200, message = 'ok') => new Response(JSON.stringify({ code: status === 200 ? 0 : 40001, data, message }), { status })
function stream(text, size = 1) {
  const bytes = encoder.encode(text)
  return new Response(new ReadableStream({ start(controller) { for (let i = 0; i < bytes.length; i += size) controller.enqueue(bytes.slice(i, i + size)); controller.close() } }), { headers: { 'Content-Type': 'text/event-stream' } })
}
const collect = async (generator) => { const list = []; for await (const event of generator) list.push(event); return list }
let expired = 0
api.configureAuth(() => 'test-token', () => expired++)
let headers
globalThis.fetch = async (_path, init) => { headers = new Headers(init.headers); return envelope(null, 401, '登录已过期') }
await assert.rejects(api.http.get('/me'), /登录已过期/)
assert.equal(expired, 1)
assert.equal(headers.get('Authorization'), 'Bearer test-token')
await assert.rejects(api.http.post('/auth/login', {}), /登录已过期/)
assert.equal(expired, 1, '错误密码不应触发已有会话的过期处理')

const events = ': 心跳\r\ndata: {"type":"citations","citations":[]}\r\n\r\ndata: {"type":"delta",\r\ndata: "text":"中文分包"}\r\n\r\ndata: {"type":"done","messageId":"saved"}\r\n\r\n'
for (const size of [1, 2, 7]) {
  globalThis.fetch = async () => stream(events, size)
  const list = await collect(api.sse('/chat', {}))
  assert.equal(list[1].text, '中文分包')
  assert.equal(list[2].messageId, 'saved')
}
globalThis.fetch = async () => stream('event: error\ndata: {"message":"生成失败"}\n\n')
await assert.rejects(collect(api.sse('/chat', {})), /生成失败/)
globalThis.fetch = async () => stream('data: {"type":"error","message":"模型不可用"}\n\n')
await assert.rejects(collect(api.sse('/chat', {})), /模型不可用/)
globalThis.fetch = async () => stream('data: {"type":"delta","text":"部分"}\n\n')
await assert.rejects(collect(api.sse('/chat', {})), /连接中断/)
let cancelled = false
globalThis.fetch = async (_path, { signal }) => new Response(new ReadableStream({ start(controller) { controller.enqueue(encoder.encode('data: {"type":"delta","text":"部分"}\n\n')); signal.addEventListener('abort', () => controller.error(new DOMException('已取消', 'AbortError'))) }, cancel() { cancelled = true } }), { headers: { 'Content-Type': 'text/event-stream' } })
const controller = new AbortController()
const gen = api.sse('/chat', {}, controller.signal)
assert.equal((await gen.next()).value.text, '部分')
controller.abort()
await assert.rejects(gen.next(), { name: 'AbortError' })
// 提前结束消费者时也必须释放连接。
globalThis.fetch = async () => new Response(new ReadableStream({ start(c) { c.enqueue(encoder.encode('data: {"type":"delta","text":"片段"}\n\n')) }, cancel() { cancelled = true } }), { headers: { 'Content-Type': 'text/event-stream' } })
const early = api.sse('/chat', {}); await early.next(); await early.return()
assert.equal(cancelled, true)

const originalTimeout = globalThis.setTimeout
const waits = []
globalThis.setTimeout = (fn, ms, ...args) => { if (ms === 1500) waits.push(ms); return originalTimeout(fn, ms === 1500 ? 1 : ms, ...args) }
const source = (id, status, error) => ({ id, notebookId: 'nb', title: id, type: 'text', status, error, createdAt: new Date().toISOString() })
const states = []
let uploadHeader
let fileNumber = 0
globalThis.XMLHttpRequest = class {
  upload = {}
  open() {}
  setRequestHeader(key, value) { if (key === 'Authorization') uploadHeader = value }
  send() { this.upload.onprogress({ lengthComputable: true, loaded: 1, total: 2 }); const id = `file-${++fileNumber}`; queueMicrotask(() => { this.status = 200; this.responseText = JSON.stringify({ code: 0, data: source(id, 'processing') }); this.onload() }) }
  abort() { this.onabort() }
}
globalThis.fetch = async (path) => envelope(source(path.split('/').pop(), 'ready'))
const file = new File(['测试'], '测试.txt')
await api.sourcesApi.add('nb', { type: 'text', title: file.name, file }, (s) => states.push([s.id, s.status, s.progress]))
assert.equal(uploadHeader, 'Bearer test-token')
assert.deepEqual(states.map((s) => s[1]), ['uploading', 'processing', 'ready'])
assert.equal(states[0][2], 50)
assert.ok(waits.length > 0 && waits.every((ms) => ms === 1500))

const workspace = api.useWorkspace
workspace.setState({ notebookId: 'nb', session: new AbortController() })
await Promise.all([workspace.getState().addSource({ type: 'text', title: '甲', file }), workspace.getState().addSource({ type: 'text', title: '乙', file })])
assert.equal(workspace.getState().sources.length, 2)
assert.equal(new Set(workspace.getState().sources.map((s) => s.id)).size, 2)
assert.ok(workspace.getState().sources.every((s) => s.status === 'ready' && !s.id.startsWith('pending')))
assert.equal(workspace.getState().selectedIds.length, 2)

globalThis.fetch = async (path) => path.endsWith('/sources') ? envelope(source('failed', 'processing')) : path.endsWith('/guide') ? envelope({ summary: '', suggestedQuestions: [] }) : envelope(source('failed', 'error', '解析失败'))
await workspace.getState().addSource({ type: 'text', title: '失败文本', text: '正文' })
assert.equal(workspace.getState().sources.find((s) => s.id === 'failed').error, '解析失败')
assert.ok(!workspace.getState().selectedIds.includes('failed'))

let audioCount = 0
globalThis.fetch = async (_path, init) => envelope({ notebookId: 'nb', status: init.method === 'POST' || audioCount++ === 0 ? 'generating' : 'ready', transcript: '对谈文稿' })
const audio = await api.studioApi.generateAudio('nb', () => {})
assert.equal(audio.status, 'ready'); assert.equal(audio.transcript, '对谈文稿')
globalThis.fetch = async () => envelope({ notebookId: 'nb', status: 'error' })
assert.equal((await api.studioApi.generateAudio('nb', () => {})).status, 'error')

// 旧流在停止后结束，不得清除刚启动的新流状态。
let streamCount = 0
const streamControllers = []
globalThis.fetch = async (_path, init) => {
  if (String(_path).endsWith('/chat')) return new Response(new ReadableStream({ start(c) { streamControllers.push(c); c.enqueue(encoder.encode(`data: {"type":"delta","text":"第${++streamCount}段"}\n\n`)); init.signal.addEventListener('abort', () => c.error(new DOMException('已取消', 'AbortError'))) } }), { headers: { 'Content-Type': 'text/event-stream' } })
  return envelope(null)
}
workspace.setState({ selectedIds: ['file-2'], messages: [] })
const first = workspace.getState().ask('第一个问题')
await new Promise((r) => originalTimeout(r, 5))
workspace.getState().stop()
assert.equal(workspace.getState().streaming, false)
const second = workspace.getState().ask('第二个问题')
await first
assert.equal(workspace.getState().streaming, true)
await new Promise((r) => originalTimeout(r, 5))
streamControllers[1].enqueue(encoder.encode('data: {"type":"done","messageId":"persisted-id"}\n\n'))
await second
assert.equal(workspace.getState().streaming, false)
assert.equal(workspace.getState().messages.at(-1).id, 'persisted-id')
assert.ok(workspace.getState().messages.every((m) => !m.streaming))
workspace.getState().reset()
globalThis.setTimeout = originalTimeout
globalThis.window = { location: { origin: 'http://localhost:5175' } }
const previewSource = { ...source('pdf-id', 'ready'), type: 'pdf', previewable: true, pageCount: 3 }
const previewUrl = new URL(api.sourceFileUrl(previewSource, 'preview', 'encoded +/?', 3))
assert.equal(previewUrl.pathname, '/api/sources/pdf-id/preview')
assert.equal(previewUrl.searchParams.get('token'), 'encoded +/?')
assert.equal(previewUrl.hash, '#page=3')
assert.equal(new URL(api.sourceFileUrl(previewSource, 'preview', null, 99)).hash, '#page=3')
assert.equal(new URL(api.sourceFileUrl(previewSource, 'preview', null, -1)).hash, '#page=1')
const fileUrl = new URL(api.sourceFileUrl(previewSource, 'file', 'token'))
assert.equal(fileUrl.pathname, '/api/sources/pdf-id/file')
assert.equal(fileUrl.hash, '')
workspace.setState({ sources: [previewSource, { ...source('sheet-id', 'ready'), type: 'xlsx', previewable: false }] })
workspace.getState().openPreview('sheet-id', 1)
assert.equal(workspace.getState().preview, null)
workspace.getState().openSource(previewSource.id, '引用片段', { page: 3, locationLabel: '第 3 页' })
assert.deepEqual(workspace.getState().citationLocation, { page: 3, locationLabel: '第 3 页' })
workspace.getState().openPreview(previewSource.id, 99)
assert.deepEqual(workspace.getState().preview, { sourceId: previewSource.id, page: 3 })
workspace.getState().closePreview()
assert.equal(workspace.getState().highlight, '引用片段', '关闭原件仍保留来源引用高亮')
workspace.getState().openPreview(previewSource.id, 2)
workspace.getState().openSource('sheet-id')
assert.equal(workspace.getState().preview, null, '切换来源时关闭旧原件')
workspace.getState().openSource(previewSource.id, '引用片段', { page: 3 })
workspace.getState().openPreview(previewSource.id, 2)
globalThis.fetch = async () => envelope(null)
await workspace.getState().removeSource(previewSource.id)
assert.equal(workspace.getState().preview, null)
assert.equal(workspace.getState().citationLocation, null)
workspace.getState().reset()
console.log('通过：原件预览/下载地址、查询参数鉴权、页码边界、不可预览表格、引用位置、关闭保留高亮、删除清理预览。')
assert.equal(new URL(api.audioFileUrl('/api/files/audio/test.mp3', 'token')).searchParams.get('token'), 'token')
assert.equal(api.audioFileUrl('https://other.example/audio.mp3', 'token'), '')
globalThis.localStorage = { getItem() { throw new Error('存储不可用') }, setItem() { throw new Error('存储不可用') }, removeItem() { throw new Error('存储不可用') } }
const auth = api.useAuthStore
const user = { id: 'user', nickname: '测试用户', email: 'test@example.com', createdAt: new Date().toISOString() }
auth.getState().accept({ token: 'memory-token', user })
assert.equal(auth.getState().token, 'memory-token')
auth.getState().logout()
assert.equal(auth.getState().user, null)
auth.setState({ token: 'restore-token', initialized: false })
api.configureAuth(() => auth.getState().token, () => auth.getState().logout())
globalThis.fetch = async () => { throw new TypeError('网络中断') }
await auth.getState().restore()
assert.equal(auth.getState().token, 'restore-token')
assert.ok(auth.getState().restoreError)
globalThis.fetch = async () => envelope(user)
await auth.getState().restore()
assert.equal(auth.getState().user.id, 'user')
auth.setState({ user: null, initialized: false })
globalThis.fetch = async () => envelope(null, 401, '登录已过期')
await auth.getState().restore()
assert.equal(auth.getState().token, null)
assert.equal(auth.getState().initialized, true)
console.log('通过：鉴权与中文错误、UTF-8/CRLF 分包、SSE 错误/断流/取消、上传鉴权与进度、并发临时来源替换、失败来源、1.5 秒轮询、音频终态、停止后新流隔离、音频鉴权地址、存储禁用、登录恢复重试及 401 清理。')
