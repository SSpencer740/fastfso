import { StrictMode, Suspense, lazy } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Routes, Route, Navigate } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ProtectedRoute } from "./components/layout/ProtectedRoute";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { Spinner } from "./components/ui/Spinner";
import { LoginPage } from "./pages/LoginPage";
import { ChallengePage } from "./pages/ChallengePage";
import { SetupTwoFactorPage } from "./pages/SetupTwoFactorPage";
import { TenantSelectPage } from "./pages/TenantSelectPage";
import { SSOCallbackPage } from "./pages/SSOCallbackPage";
import { AcceptInvitePage } from "./pages/AcceptInvitePage";
import { ForgotPasswordPage } from "./pages/ForgotPasswordPage";
import { ResetPasswordPage } from "./pages/ResetPasswordPage";
import { AppInitializer } from "./AppInitializer";
import { AppShell } from "./components/layout/AppShell";
import { AppIndexRedirect } from "./components/layout/AppIndexRedirect";
import { initErrorReporting } from "./lib/errorReporter";
import "./index.scss";

initErrorReporting();

const queryClient = new QueryClient();

// Lazy-loaded app pages
const DashboardPage = lazy(() => import("./pages/app/DashboardPage"));
const TasksPage = lazy(() => import("./pages/app/TasksPage"));
const ReportsPage = lazy(() => import("./pages/app/ReportsPage"));
const TravelPage = lazy(() => import("./pages/app/TravelPage"));
const VisitsPage = lazy(() => import("./pages/app/VisitsPage"));
const TeamPage = lazy(() => import("./pages/app/TeamPage"));
const Dd254Page = lazy(() => import("./pages/app/Dd254Page"));
const Dd254DetailPage = lazy(() => import("./pages/app/Dd254DetailPage"));
const WikiPage = lazy(() => import("./pages/app/WikiPage"));
const CustomerAdminPage = lazy(() => import("./pages/app/CustomerAdminPage"));
const ActionItemsPage = lazy(() => import("./pages/app/ActionItemsPage"));
const SettingsPage = lazy(() => import("./pages/app/SettingsPage"));
const ChatPage = lazy(() => import("./pages/app/ChatPage"));
const AuditLogPage = lazy(() => import("./pages/app/AuditLogPage"));

// Lazy-loaded: admin panel is a separate bundle, only fetched when accessed.
const AdminPanelPage = lazy(() => import("./pages/AdminPanelPage"));

const AppFallback = <Spinner fullPage />;

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
    <ErrorBoundary>
      <BrowserRouter>
        <AppInitializer />
        <Routes>
          <Route
            path="/login"
            element={
              <ProtectedRoute allowedStates={["unauthenticated"]}>
                <LoginPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/challenge"
            element={
              <ProtectedRoute allowedStates={["pre_auth"]}>
                <ChallengePage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/setup-2fa"
            element={
              <ProtectedRoute allowedStates={["setup_2fa"]}>
                <SetupTwoFactorPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/select-tenant"
            element={
              <ProtectedRoute allowedStates={["pre_tenant"]}>
                <TenantSelectPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/app"
            element={
              <ProtectedRoute allowedStates={["authenticated"]}>
                <AppShell />
              </ProtectedRoute>
            }
          >
            <Route index element={<AppIndexRedirect />} />
            <Route path="dashboard" element={<Suspense fallback={AppFallback}><DashboardPage /></Suspense>} />
            <Route path="tasks" element={<Suspense fallback={AppFallback}><TasksPage /></Suspense>} />
            <Route path="action-items" element={<Suspense fallback={AppFallback}><ActionItemsPage /></Suspense>} />
            <Route path="reports" element={<Suspense fallback={AppFallback}><ReportsPage /></Suspense>} />
            <Route path="travel" element={<Suspense fallback={AppFallback}><TravelPage /></Suspense>} />
            <Route path="visits" element={<Suspense fallback={AppFallback}><VisitsPage /></Suspense>} />
            <Route path="team" element={<Suspense fallback={AppFallback}><TeamPage /></Suspense>} />
            <Route path="dd254" element={<Suspense fallback={AppFallback}><Dd254Page /></Suspense>} />
            <Route path="dd254/:id" element={<Suspense fallback={AppFallback}><Dd254DetailPage /></Suspense>} />
            <Route path="wiki" element={<Suspense fallback={AppFallback}><WikiPage /></Suspense>} />
            <Route path="admin" element={<Suspense fallback={AppFallback}><CustomerAdminPage /></Suspense>} />
            <Route path="audit-log" element={<Suspense fallback={AppFallback}><AuditLogPage /></Suspense>} />
            <Route path="settings" element={<Suspense fallback={AppFallback}><SettingsPage /></Suspense>} />
            <Route path="chat" element={<Suspense fallback={AppFallback}><ChatPage /></Suspense>} />
          </Route>
          <Route
            path="/admin/*"
            element={
              <ProtectedRoute allowedStates={["authenticated"]}>
                <Suspense fallback={<Spinner fullPage />}>
                  <AdminPanelPage />
                </Suspense>
              </ProtectedRoute>
            }
          />
          <Route
            path="/settings"
            element={<Navigate to="/app/settings" replace />}
          />
          <Route path="/accept-invite" element={<AcceptInvitePage />} />
          <Route path="/forgot-password" element={<ForgotPasswordPage />} />
          <Route path="/reset-password" element={<ResetPasswordPage />} />
          <Route path="/sso/callback" element={<SSOCallbackPage />} />
          <Route path="*" element={<Navigate to="/login" replace />} />
        </Routes>
      </BrowserRouter>
    </ErrorBoundary>
    </QueryClientProvider>
  </StrictMode>,
);
