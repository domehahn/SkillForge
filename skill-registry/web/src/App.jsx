import { Routes, Route } from 'react-router-dom'
import Header from './components/Header'
import HomePage from './pages/HomePage'
import SkillDetailPage from './pages/SkillDetailPage'
import ArtifactDetailPage from './pages/ArtifactDetailPage'

function App() {
  return (
    <div className="app">
      <Header />
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/skills/:namespace/:name" element={<SkillDetailPage />} />
        <Route path="/artifacts/:kind/:namespace/:name" element={<ArtifactDetailPage />} />
      </Routes>
    </div>
  )
}

export default App
