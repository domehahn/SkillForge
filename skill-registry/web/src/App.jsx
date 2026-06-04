import { Routes, Route } from 'react-router-dom'
import Header from './components/Header'
import HomePage from './pages/HomePage'
import SkillDetailPage from './pages/SkillDetailPage'

function App() {
  return (
    <div className="app">
      <Header />
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/skills/:namespace/:name" element={<SkillDetailPage />} />
      </Routes>
    </div>
  )
}

export default App
