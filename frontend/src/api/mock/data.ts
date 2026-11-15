import type { Notebook, Source, Note, StudioArtifact, Message, UserSettings, Tag } from '@/types'
import { pdfSections, slideSections, sheetSections, sectionContent } from './sourceFixtures'

const ago = (h: number) => new Date(Date.now() - h * 3600_000).toISOString()

export const tags: Tag[] = [
  { id: 'tag-study', name: '学习', color: '#6750A4', createdAt: ago(240) },
  { id: 'tag-research', name: '科研', color: '#146C2E', createdAt: ago(240) },
]
export const notebooks: Notebook[] = [
  { id: 'nb_1', title: '大语言模型与 RAG 技术综述', emoji: '🧠', color: 0, description: '毕业设计相关论文与技术资料', tags: [tags[0], tags[1]], sourceCount: 6, createdAt: ago(240), updatedAt: ago(1) },
  { id: 'nb_2', title: '数据库系统原理复习', emoji: '🗄️', color: 3, tags: [tags[0]], sourceCount: 3, createdAt: ago(500), updatedAt: ago(26) },
  { id: 'nb_3', title: '产品经理面试准备', emoji: '💼', color: 5, tags: [], sourceCount: 2, createdAt: ago(800), updatedAt: ago(72) },
  { id: 'nb_4', title: '机器学习课程笔记', emoji: '📈', color: 2, tags: [tags[0]], sourceCount: 3, createdAt: ago(1200), updatedAt: ago(150) },
]

export const sources: Source[] = [
  {
    id: 'src_1', notebookId: 'nb_1', type: 'pdf', title: 'RAG 阅读与引用指南.pdf',
    status: 'ready', size: 248_000, wordCount: 1260, createdAt: ago(200),
    originalExt: 'pdf', previewable: true, pageCount: 3, sections: pdfSections,
    summary: '这份阅读指南介绍检索增强生成的基本流程，并说明如何通过来源目录和回答中的引用回到具体页面。三页内容分别聚焦研究背景、资料处理流水线和引用的可追溯性，可结合原件预览逐页阅读。',
    topics: ['检索增强生成', '文档切分', '原件预览', '引用定位'],
    content: sectionContent(pdfSections),
  },
  {
    id: 'src_2', notebookId: 'nb_1', type: 'web', title: 'What is RAG? - 向量数据库与检索增强生成入门', url: 'https://example.com/what-is-rag',
    status: 'ready', wordCount: 3200, createdAt: ago(180),
    summary: '一篇面向工程实践的入门文章，介绍了 RAG 系统的典型流水线：文档切分（Chunking）、嵌入（Embedding）、向量存储、相似度检索与重排序（Rerank），并讨论了 chunk 大小、重叠窗口等参数对召回率的影响。',
    topics: ['文档切分', 'Embedding', '向量数据库', '重排序', '召回率'],
    content: 'RAG 的核心思想是：在大模型回答问题之前，先从外部知识库中检索出相关内容，再把这些内容作为上下文交给模型。典型流程包括：\n\n1. 文档加载与切分：将 PDF、网页等资料切分为 300~800 字的片段；\n2. 向量化：使用嵌入模型将片段编码为向量；\n3. 存储：写入 Milvus、pgvector、Chroma 等向量数据库；\n4. 检索：将用户问题向量化后进行 Top-K 相似度搜索；\n5. 生成：将检索结果拼接进 Prompt，由大模型生成带引用的回答。',
  },
  {
    id: 'src_3', notebookId: 'nb_1', type: 'docx', title: '毕业设计开题报告.docx',
    status: 'ready', size: 386_000, wordCount: 6400, createdAt: ago(120),
    summary: '开题报告阐述了“基于大语言模型的个人资料管理系统”的研究背景、国内外现状、研究内容与技术路线。系统计划支持多格式资料导入、基于 RAG 的智能问答、自动摘要与思维导图生成，前端采用 React + TypeScript，后端采用 Go + Gin + PostgreSQL/pgvector。',
    topics: ['研究背景', '技术路线', '系统架构', '进度安排'],
    content: '一、选题背景与意义\n随着信息爆炸式增长，个人用户积累了大量论文、笔记、网页收藏等资料，传统的文件夹式管理难以实现高效检索与知识整合……\n\n二、研究内容\n（1）多源异构资料的解析与统一存储；（2）基于向量检索的语义问答；（3）知识自动整理：摘要、FAQ、时间线、思维导图……',
  },
  {
    id: 'src_4', notebookId: 'nb_1', type: 'youtube', title: 'Andrej Karpathy - Intro to Large Language Models', url: 'https://youtube.com/watch?v=example',
    status: 'ready', wordCount: 8800, createdAt: ago(60),
    summary: '一小时的大语言模型科普讲座，介绍了 LLM 的预训练、微调与 RLHF 流程，讨论了 Scaling Law、工具使用、多模态以及“LLM 操作系统”的类比，最后梳理了越狱、提示注入等安全问题。',
    topics: ['预训练', 'RLHF', 'Scaling Law', 'LLM OS', '提示注入'],
    content: '（视频转写）大语言模型本质上只是两个文件：一个参数文件和一段运行这些参数的代码……',
  },
  {
    id: 'src_15', notebookId: 'nb_1', type: 'pptx', title: '知页系统设计与答辩.pptx',
    status: 'ready', size: 1_680_000, wordCount: 2180, createdAt: ago(50), originalExt: 'pptx',
    pageCount: 6, sections: slideSections, previewable: true,
    summary: '六张幻灯片介绍知页的研究背景、系统架构、资料解析、检索引用、学习工作流与验证计划。系统按资料结构保存位置，让阅读者从回答中的证据返回原件核对上下文。',
    topics: ['系统架构', '多格式来源', '结构化目录', '引用定位'], content: sectionContent(slideSections),
  },
  {
    id: 'src_16', notebookId: 'nb_1', type: 'xlsx', title: '资料与检索实验记录.xlsx',
    status: 'ready', size: 32_000, wordCount: 860, createdAt: ago(40), originalExt: 'xlsx',
    pageCount: 2, sections: sheetSections, previewable: false,
    summary: '表格按工作表记录检索实验参数与来源类型。检索实验使用 500 字片段和 80 字重叠，并保留每个片段的页码；来源统计说明 PDF、幻灯片与工作表的目录粒度。',
    topics: ['检索实验', '切分参数', '来源统计'], content: sectionContent(sheetSections),
  },
  { id: 'src_5', notebookId: 'nb_2', type: 'pdf', title: '数据库系统概论（第6版）第三章.pdf', status: 'ready', size: 5_200_000, wordCount: 21000, createdAt: ago(500), summary: '介绍关系数据库标准语言 SQL，包括数据定义、查询、更新、视图等。', topics: ['SQL', '视图', '嵌套查询'], content: 'SQL 是结构化查询语言……' },
  { id: 'src_6', notebookId: 'nb_2', type: 'markdown', title: '事务与并发控制笔记.md', status: 'ready', size: 18_000, wordCount: 2600, createdAt: ago(480), summary: '总结了 ACID 特性、封锁协议、两段锁与 MVCC。', topics: ['ACID', '两段锁', 'MVCC'], content: '# 事务\n事务是用户定义的一个数据库操作序列……' },
  { id: 'src_7', notebookId: 'nb_2', type: 'text', title: '往年真题整理', status: 'ready', wordCount: 1500, createdAt: ago(470), summary: '整理了近五年期末考试高频考点。', topics: ['范式', 'E-R 图'], content: '1. 试述关系模型的三类完整性约束……' },
  { id: 'src_8', notebookId: 'nb_3', type: 'pdf', title: '产品经理面试高频问题 100 问.pdf', status: 'ready', size: 1_200_000, wordCount: 12000, createdAt: ago(800), summary: '汇总了产品经理面试中关于需求分析、竞品分析、数据指标与项目管理的高频问题及参考回答。', topics: ['需求分析', '竞品分析', '数据指标', 'PRD'], content: '1. 请介绍一款你最喜欢的产品，并说明理由……' },
  { id: 'src_9', notebookId: 'nb_3', type: 'web', title: '网页：如何写好一份 PRD', url: 'https://example.com/prd', status: 'ready', wordCount: 4100, createdAt: ago(780), summary: '介绍 PRD 的结构：背景、目标、用户故事、功能需求、非功能需求与验收标准。', topics: ['PRD', '用户故事', '验收标准'], content: '一份好的 PRD 应当让研发、设计、测试都能读懂……' },
  { id: 'src_10', notebookId: 'nb_4', type: 'pdf', title: '吴恩达机器学习讲义 第1-3周.pdf', status: 'ready', size: 3_600_000, wordCount: 15000, createdAt: ago(1200), summary: '涵盖线性回归、梯度下降、逻辑回归与正则化。', topics: ['线性回归', '梯度下降', '逻辑回归', '正则化'], content: '监督学习是指给定带标签的训练数据……' },
  { id: 'src_11', notebookId: 'nb_4', type: 'markdown', title: '神经网络反向传播推导.md', status: 'ready', size: 22_000, wordCount: 3000, createdAt: ago(1100), summary: '逐步推导了多层感知机中的反向传播公式。', topics: ['反向传播', '链式法则', '激活函数'], content: '# 反向传播\n设损失函数为 L……' },
  { id: 'src_12', notebookId: 'nb_4', type: 'youtube', title: '视频：3Blue1Brown 神经网络系列', url: 'https://youtube.com/watch?v=example2', status: 'ready', wordCount: 7000, createdAt: ago(1000), summary: '以可视化方式直观讲解神经网络、梯度下降与反向传播。', topics: ['神经网络', '可视化', '梯度下降'], content: '（视频转写）什么是神经网络？……' },
]

export const notes: Note[] = [
  {
    id: 'note_1', notebookId: 'nb_1', title: 'RAG 与微调的区别', origin: 'chat', createdAt: ago(20), updatedAt: ago(20),
    content: '- **RAG**：不修改模型参数，通过检索外部知识实时补充上下文，知识可随时更新，可提供引用来源。\n- **微调**：修改模型参数，适合学习风格与格式，知识更新成本高。\n- 毕设中采用 RAG 方案，更符合“个人资料随时增删”的场景。',
  },
  {
    id: 'note_2', notebookId: 'nb_1', title: '答辩可能被问到的问题', origin: 'manual', createdAt: ago(5), updatedAt: ago(3),
    content: '1. 为什么选择向量检索而不是关键词检索？\n2. 如何保证回答不“幻觉”？——强制引用来源\n3. 文档切分策略如何确定？',
  },
]

export const artifacts: StudioArtifact[] = []

export const messages: Record<string, Message[]> = { nb_1: [] }

export const settings: UserSettings = {
  model: 'qwen-max',
  answerStyle: 'detailed',
  language: 'zh-CN',
  storageUsed: 1.38 * 1024 ** 3,
  storageTotal: 5 * 1024 ** 3,
}
