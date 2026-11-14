import type { ChatStreamEvent, Message } from '@/types'
import { USE_MOCK, http, sse } from './client'
import * as mock from './mock/handlers'

export const chatApi = {
  history: (notebookId: string): Promise<Message[]> =>
    USE_MOCK ? mock.listMessages(notebookId) : http.get(`/notebooks/${notebookId}/messages`),
  clear: (notebookId: string): Promise<void> =>
    USE_MOCK ? mock.clearMessages(notebookId) : http.del(`/notebooks/${notebookId}/messages`),
  /** 流式问答：POST /notebooks/:id/chat  (text/event-stream) */
  stream: (notebookId: string, question: string, sourceIds: string[], signal?: AbortSignal): AsyncGenerator<ChatStreamEvent> =>
    USE_MOCK
      ? mock.streamChat(notebookId, question, sourceIds, signal)
      : sse<ChatStreamEvent>(`/notebooks/${notebookId}/chat`, { question, sourceIds }, signal),
  feedback: (messageId: string, value: 'up' | 'down'): Promise<void> =>
    USE_MOCK ? Promise.resolve() : http.post(`/messages/${messageId}/feedback`, { value }),
}
