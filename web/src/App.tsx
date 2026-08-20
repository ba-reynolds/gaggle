import { lazy, Suspense } from 'react';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import AuthProvider from '@/contexts/AuthContext';
import { UserProvider } from './contexts/UserContext';
import { Toaster } from './components/ui/sonner';
import { ThemeProvider } from './contexts/ThemeContext';
import { I18nProvider } from './contexts/I18nContext';
import AppearanceSync from './components/AppearanceSync';
import SocialMediaLayout from '@/layout/SocialMediaLayout';
import AppSplash from '@/components/AppSplash';
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';
import { NotificationsProvider } from './contexts/NotificationsContext';

const FeedPage = lazy(() => import('./pages/FeedPage'));
const LoginPage = lazy(() => import('./pages/LoginPage'));
const SignupPage = lazy(() => import('./pages/SignupPage'));
const ProfilePage = lazy(() => import('./pages/ProfilePage'));
const FollowListPage = lazy(() => import('./pages/FollowListPage'));
const BookmarksPage = lazy(() => import('./pages/BookmarksPage'));
const SettingsPage = lazy(() => import('./pages/SettingsPage'));
const PostPage = lazy(() => import('./pages/PostPage'));
const NotificationsPage = lazy(() => import('./pages/NotificationsPage'));
const MentionsPage = lazy(() => import('./pages/MentionsPage'));
const SearchPage = lazy(() => import('./pages/SearchPage'));
const HashtagPage = lazy(() => import('./pages/HashtagPage'));
const AdminPage = lazy(() => import('./pages/AdminPage'));
const ExplorePage = lazy(() => import('./pages/ExplorePage'));
const ListsPage = lazy(() => import('./pages/ListsPage'));
const ListPage = lazy(() => import('./pages/ListPage'));
const MessagesPage = lazy(() => import('./pages/MessagesPage'));
const ConversationPage = lazy(() => import('./pages/ConversationPage'));
const LoginLabPage = lazy(() =>
  import('./pages/login-lab/LoginLabPage').then((m) => ({ default: m.LoginLabPage })),
);


const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
      refetchOnWindowFocus: false,
      // Keep fetched pages fresh for a minute so navigating back to a page
      // within that window is served from cache instead of refetching on every
      // mount. Pages that need fresher data override this per-query.
      staleTime: 60_000,
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
              <I18nProvider>
                <AppearanceSync />
                <Router>
                  <Suspense fallback={<AppSplash />}>
                    <Routes>
                      <Route path="/" element={<SocialMediaLayout><FeedPage /></SocialMediaLayout>} />
                      <Route path="/login" element={<LoginPage />} />
                      <Route path="/login-lab" element={<LoginLabPage />} />
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
                      <Route path="/messages/new" element={<SocialMediaLayout><ConversationPage /></SocialMediaLayout>} />
                      <Route path="/messages/:conversationId" element={<SocialMediaLayout><ConversationPage /></SocialMediaLayout>} />
                      <Route path="/search" element={<SocialMediaLayout><SearchPage /></SocialMediaLayout>} />
                      <Route path="/hashtags/:tag" element={<SocialMediaLayout><HashtagPage /></SocialMediaLayout>} />
                      <Route path="/admin" element={<SocialMediaLayout><AdminPage /></SocialMediaLayout>} />
                    </Routes>
                  </Suspense>
                </Router>
              </I18nProvider>
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
