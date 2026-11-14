import type { Source, SourceSection, SourceType } from '@/types'

export const pdfSections: SourceSection[] = [
  { page: 1, label: '第 1 页', title: '为什么需要 RAG', excerpt: '检索增强生成将外部资料与语言模型结合，让回答能够回到具体来源。个人知识库可以随资料增删而更新，无需重新训练模型。' },
  { page: 2, label: '第 2 页', title: '从资料到回答', excerpt: '资料先按页提取正文，再切分为语义片段并向量化。提问时检索相关片段，将其作为上下文，生成带引用的回答。' },
  { page: 3, label: '第 3 页', title: '引用与可追溯性', excerpt: '每条引用保留来源、原文片段和页码。阅读者可以从回答跳回资料的对应页面，结合上下文核对结论，避免把模型输出当成唯一依据。' },
]

export const slideSections: SourceSection[] = [
  { page: 1, label: '第 1 张幻灯片：研究背景', title: '研究背景', excerpt: '个人资料分散在论文、演示文稿和表格中。知页将多格式资料汇集到笔记本，帮助使用者阅读、提问与整理知识。' },
  { page: 2, label: '第 2 张幻灯片：系统架构', title: '系统架构', excerpt: '前端使用 React 与 TypeScript，后端使用 Go。PostgreSQL 和 pgvector 统一保存业务数据与向量，减少部署组件。' },
  { page: 3, label: '第 3 张幻灯片：资料解析', title: '资料解析', excerpt: 'PDF 按页解析，演示文稿按幻灯片解析，表格按工作表解析。每个结构单元保留标题、摘要和序号，形成可浏览的目录。' },
  { page: 4, label: '第 4 张幻灯片：检索与引用', title: '检索与引用', excerpt: '文档分块不跨页、不跨幻灯片或工作表。检索片段保留原始位置，回答中的引用可以直接定位到原件，便于核对上下文。' },
  { page: 5, label: '第 5 张幻灯片：学习工作流', title: '学习工作流', excerpt: '从引用回看原件，确认内容后保存到笔记。学习指南、简报和思维导图帮助使用者进一步整理资料与复习。' },
  { page: 6, label: '第 6 张幻灯片：验证与展望', title: '验证与展望', excerpt: '通过多格式资料导入、带引用问答与页码定位验证阅读流程。后续可结合真实学习资料评估检索质量与知识整理效果。' },
]

export const sheetSections: SourceSection[] = [
  { page: 1, label: '工作表：检索实验', title: '检索实验', excerpt: '方案：按结构切分；片段长度：500 字；重叠：80 字；说明：页码随片段保留，可用于回答的引用定位。' },
  { page: 2, label: '工作表：来源统计', title: '来源统计', excerpt: '资料类型：PDF、PPTX、XLSX；目录粒度：页、幻灯片、工作表；阅读方式：来源概览、原文摘录与可用的原件预览。' },
]

export function mockSourceStructure(type: SourceType): Pick<Source, 'pageCount' | 'sections' | 'previewable'> {
  const sections = type === 'pptx' ? slideSections : type === 'pdf' || type === 'docx' ? pdfSections : type === 'xlsx' || type === 'csv' ? sheetSections : undefined
  return { pageCount: sections?.length, sections, previewable: type === 'pdf' || type === 'pptx' || type === 'docx' }
}

export const sectionContent = (sections: SourceSection[]) => sections.map((s) => `${s.label}\n${s.excerpt}`).join('\n\n')
