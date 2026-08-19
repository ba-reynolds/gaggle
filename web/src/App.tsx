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
import FollowListPage from './pages/FollowListPage';
import BookmarksPage from './pages/BookmarksPage';
import SettingsPage from './pages/SettingsPage';
import PostPage from './pages/PostPage';
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';
import { NotificationsProvider } from './contexts/NotificationsContext';
import NotificationsPage from './pages/NotificationsPage';
import MentionsPage from './pages/MentionsPage';
import SearchPage from './pages/SearchPage';
import HashtagPage from './pages/HashtagPage';
import AdminPage from './pages/AdminPage';
import ExplorePage from './pages/ExplorePage';
import ListsPage from './pages/ListsPage';
import ListPage from './pages/ListPage';
import MessagesPage from './pages/MessagesPage';
import ConversationPage from './pages/ConversationPage';


const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
      refetchOnWindowFocus: false,
    },
    mutations: {
      retry: false,
    },
  },
});

function App() {
  return (
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <UserProvider>
          <AuthProvider>
            <NotificationsProvider>
              <Router>
                <Routes>
                  <Route path="/" element={<SocialMediaLayout><FeedPage /></SocialMediaLayout>} />
                  <Route path="/login" element={<LoginPage />} />
                  <Route path="/signup" element={<SignupPage />} />
                  <Route path="/profile/:username" element={<SocialMediaLayout><ProfilePage /></SocialMediaLayout>} />
                  <Route path="/profile/:username/followers" element={<SocialMediaLayout><FollowListPage listType="followers" /></SocialMediaLayout>} />
                  <Route path="/profile/:username/following" element={<SocialMediaLayout><FollowListPage listType="following" /></SocialMediaLayout>} />
                  <Route path="/bookmarks" element={<SocialMediaLayout><BookmarksPage /></SocialMediaLayout>} />
                  <Route path="/lists" element={<SocialMediaLayout><ListsPage /></SocialMediaLayout>} />
                  <Route path="/lists/:id" element={<SocialMediaLayout><ListPage /></SocialMediaLayout>} />
                  <Route path="/explore" element={<SocialMediaLayout><ExplorePage /></SocialMediaLayout>} />
                  <Route path="/settings" element={<SocialMediaLayout><SettingsPage /></SocialMediaLayout>} />
                  <Route path="/post/:id" element={<SocialMediaLayout><PostPage /></SocialMediaLayout>} />
                  <Route path="/notifications" element={<SocialMediaLayout><NotificationsPage /></SocialMediaLayout>} />
                  <Route path="/mentions" element={<SocialMediaLayout><MentionsPage /></SocialMediaLayout>} />
                  <Route path="/messages" element={<SocialMediaLayout><MessagesPage /></SocialMediaLayout>} />
                  <Route path="/messages/:conversationId" element={<SocialMediaLayout><ConversationPage /></SocialMediaLayout>} />
                  <Route path="/search" element={<SocialMediaLayout><SearchPage /></SocialMediaLayout>} />
                  <Route path="/hashtags/:tag" element={<SocialMediaLayout><HashtagPage /></SocialMediaLayout>} />
                  <Route path="/admin" element={<SocialMediaLayout><AdminPage /></SocialMediaLayout>} />
                </Routes>
              </Router>
            </NotificationsProvider>
          </AuthProvider>
        </UserProvider>
        <Toaster />
        <ReactQueryDevtools initialIsOpen={false} />
      </QueryClientProvider>
    </ThemeProvider>
  );
}

export default App;
