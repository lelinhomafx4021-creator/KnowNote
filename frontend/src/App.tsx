import { BrowserRouter, Route, Routes } from 'react-router-dom'
import HomePage from '@/pages/HomePage'
import NotebookPage from '@/pages/NotebookPage'
import NotFound from '@/pages/NotFound'
import { Toaster, ConfirmHost, SettingsModal } from '@/components/layout/Overlays'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/notebook/:id" element={<NotebookPage />} />
        <Route path="*" element={<NotFound />} />
      </Routes>
      <Toaster />
      <ConfirmHost />
      <SettingsModal />
    </BrowserRouter>
  )
}
