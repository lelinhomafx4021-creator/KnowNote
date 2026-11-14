import type { DailyUsage, ModelCatalog } from '@/types'
import { USE_MOCK, http } from './client'

const demoModels: ModelCatalog = {
  chat: [
    { id: 'qwen-max', name: '通义千问 Max', provider: '综合能力强，适合长文档问答' },
    { id: 'deepseek-v3', name: 'DeepSeek V3', provider: '推理能力出色，性价比高' },
    { id: 'glm-4', name: 'GLM-4', provider: '中文理解好，响应速度快' },
    { id: 'local-llama', name: '本地模型（Ollama）', provider: '数据不出本机，隐私优先' },
  ],
  embed: { model: '演示向量', dimension: 1536, provider: 'fake' },
  tts: { enabled: false }, image: { enabled: false, provider: '', model: '' },
}
export const modelsApi = {
  get: (): Promise<ModelCatalog> => USE_MOCK ? Promise.resolve(demoModels) : http.get('/models'),
  usage: (): Promise<DailyUsage> => USE_MOCK
    ? Promise.resolve({ date: new Date().toLocaleDateString('sv-SE'), chatRequests: 12, tokens: 8400, reservedTokens: 0, chatRequestsLimit: 200, tokensLimit: 200000 })
    : http.get('/me/usage'),
}
