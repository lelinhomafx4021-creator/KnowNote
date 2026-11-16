// 拖拽产物时使用的自定义数据类型，只携带产物 id，避免与普通文字拖拽混淆
export const ARTIFACT_DRAG_TYPE = 'application/x-knownote-artifact'

export function isArtifactDrag(e: { dataTransfer: DataTransfer }): boolean {
  return Array.from(e.dataTransfer.types).includes(ARTIFACT_DRAG_TYPE)
}
