import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import AuthProvider from '@/contexts/AuthContext';
import LoginPage from '@/pages/LoginPage';
import SignupPage from '@/pages/SignupPage';
import { UserProvider } from './contexts/UserContext';
import { Toaster } from './components/ui/sonner';
import { ThemeProvider } from './contexts/ThemeContext';
import SocialMediaLayout from '@/layout/SocialMediaLayout';
import FeedPage from './pages/FeedPage';
import ProfilePage from './pages/ProfilePage';
import BookmarksPage from './pages/BookmarksPage';
import SettingsPage from './pages/SettingsPage';
import PostPage from './pages/PostPage';
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';


const queryClient = new QueryClient();

function App() {
  return (
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <UserProvider>
          <AuthProvider>
            <Router>
              <Routes>
                <Route path="/" element={<SocialMediaLayout><FeedPage /></SocialMediaLayout>} />
                <Route path="/login" element={<LoginPage />} />
                <Route path="/signup" element={<SignupPage />} />
                <Route path="/profile/:username" element={<SocialMediaLayout><ProfilePage /></SocialMediaLayout>} />
                <Route path="/bookmarks" element={<SocialMediaLayout><BookmarksPage /></SocialMediaLayout>} />
                <Route path="/settings" element={<SocialMediaLayout><SettingsPage /></SocialMediaLayout>} />
                <Route path="/post/:id" element={<SocialMediaLayout><PostPage /></SocialMediaLayout>} />
              </Routes>
            </Router>
          </AuthProvider>
        </UserProvider>
        <Toaster />
        <ReactQueryDevtools initialIsOpen={false} />
      </QueryClientProvider>
    </ThemeProvider>
  );
}

export default App;
