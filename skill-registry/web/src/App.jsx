import { Routes, Route } from 'react-router-dom'
import Header from './components/Header'
import Footer from './components/Footer'
import ScrollToTop from './components/ScrollToTop'
import LoadingBar from './components/LoadingBar'
import BackToTop from './components/BackToTop'
import ErrorBoundary from './components/ErrorBoundary'
import { ToastProvider } from './contexts/ToastContext'
import { ConfirmProvider } from './contexts/ConfirmContext'
import { ThemeProvider } from './contexts/ThemeContext'
import HomePage from './pages/HomePage'
import ArtifactDetailPage from './pages/ArtifactDetailPage'
import NamespacePage from './pages/NamespacePage'
import LoginPage from './pages/LoginPage'
import TokensPage from './pages/TokensPage'
import NotFoundPage from './pages/NotFoundPage'
import PublishPage from './pages/PublishPage'
import AdminPage from './pages/AdminPage'
import WebhooksPage from './pages/WebhooksPage'
import AccountSettingsPage from './pages/AccountSettingsPage'
import NamespaceSettingsPage from './pages/NamespaceSettingsPage'
import ActivityPage from './pages/ActivityPage'
import ExplorePage from './pages/ExplorePage'
import InsightsPage from './pages/InsightsPage'
import CollectionsPage, { CollectionDetailPage } from './pages/CollectionsPage'
import SearchPage from './pages/SearchPage'
import UserProfilePage from './pages/UserProfilePage'
import AuditLogPage from './pages/AuditLogPage'
import NotificationPrefsPage from './pages/NotificationPrefsPage'
import InstallPage from './pages/InstallPage'
import CategoriesPage from './pages/CategoriesPage'
import AccountSecurityPage from './pages/AccountSecurityPage'
import TrendingPage from './pages/TrendingPage'

export default function App() {
  return (
    <ErrorBoundary>
      <ThemeProvider>
      <ToastProvider>
        <ConfirmProvider>
          <div className="app">
            <LoadingBar />
            <ScrollToTop />
            <Header />
            <main className="app-main">
              <Routes>
                <Route path="/" element={<HomePage />} />
                <Route path="/artifacts/:kind/:namespace/:name" element={<ArtifactDetailPage />} />
                <Route path="/namespace/:namespace" element={<NamespacePage />} />
                <Route path="/namespace/:namespace/webhooks" element={<WebhooksPage />} />
                <Route path="/namespace/:namespace/settings" element={<NamespaceSettingsPage />} />
                <Route path="/login" element={<LoginPage />} />
                <Route path="/account/tokens" element={<TokensPage />} />
                <Route path="/account/settings" element={<AccountSettingsPage />} />
                <Route path="/publish" element={<PublishPage />} />
                <Route path="/admin" element={<AdminPage />} />
                <Route path="/activity" element={<ActivityPage />} />
                <Route path="/explore" element={<ExplorePage />} />
                <Route path="/namespace/:namespace/insights" element={<InsightsPage />} />
                <Route path="/namespace/:namespace/collections" element={<CollectionsPage />} />
                <Route path="/namespace/:owner/collections/:slug" element={<CollectionDetailPage />} />
                <Route path="/search" element={<SearchPage />} />
                <Route path="/u/:username" element={<UserProfilePage />} />
                <Route path="/admin/audit" element={<AuditLogPage />} />
                <Route path="/account/notifications" element={<NotificationPrefsPage />} />
                <Route path="/account/security" element={<AccountSecurityPage />} />
                <Route path="/install" element={<InstallPage />} />
                <Route path="/categories" element={<CategoriesPage />} />
                <Route path="/trending" element={<TrendingPage />} />
                <Route path="*" element={<NotFoundPage />} />
              </Routes>
            </main>
            <Footer />
            <BackToTop />
          </div>
        </ConfirmProvider>
      </ToastProvider>
      </ThemeProvider>
    </ErrorBoundary>
  )
}
