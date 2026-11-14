import { Link } from 'react-router-dom'
import { Compass } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { EmptyState } from '@/components/ui/misc'

export default function NotFound() {
  return (
    <div className="flex h-full items-center justify-center">
      <EmptyState icon={<Compass />} title="页面不存在" description="你访问的页面可能已被移动或删除。" action={<Link to="/"><Button>返回首页</Button></Link>} />
    </div>
  )
}
