import React, { Suspense, lazy } from 'react';
import { BrowserRouter as Router, Route, Routes, Navigate } from 'react-router-dom';
import { AuthProvider } from './contexts/AuthContext';
import { AppsProvider } from './contexts/AppsContext';
import { ThemeProvider } from './contexts/ThemeContext';
import ProtectedRoute from './components/ProtectedRoute';
import AuthLayout from './layouts/AuthLayout';
import DashboardLayout from './layouts/DashboardLayout';
import ErrorBoundary from './components/ErrorBoundary';
import { Toaster } from './components/ui/sonner';
import './index.css'

// Lazy-loaded pages
const LandingPage = lazy(() => import('./pages/LandingPage'));
const Login = lazy(() => import('./pages/Login'));
const Register = lazy(() => import('./pages/Register'));
const ForgotPassword = lazy(() => import('./pages/ForgotPassword'));
const ResetPassword = lazy(() => import('./pages/ResetPassword'));
const SSOCallback = lazy(() => import('./pages/SSOCallback'));
const AppsList = lazy(() => import('./pages/AppsList'));
const TenantsList = lazy(() => import('./pages/TenantsList'));
const TenantDetail = lazy(() => import('./pages/TenantDetail'));
const AppDetail = lazy(() => import('./pages/AppDetail'));
const Dashboard = lazy(() => import('./pages/Dashboard'));
const WorkflowsList = lazy(() => import('./pages/workflows/WorkflowsList'));
const WorkflowBuilder = lazy(() => import('./pages/workflows/WorkflowBuilder'));
const WorkflowExecutions = lazy(() => import('./pages/workflows/WorkflowExecutions'));
const DigestRulesList = lazy(() => import('./pages/digest/DigestRulesList'));
const TopicsList = lazy(() => import('./pages/topics/TopicsList'));
const AuditLogsList = lazy(() => import('./pages/audit/AuditLogsList'));
const TemplateLibrary = lazy(() => import('./pages/TemplateLibrary'));
const DocsLayout = lazy(() => import('./pages/docs/DocsLayout'));
const DocsPage = lazy(() => import('./pages/docs/DocsPage'));
const ApiReferencePage = lazy(() => import('./pages/docs/ApiReferencePage'));

// Eager-loaded global components
import CommandPalette from './components/CommandPalette';

// Loading fallback (shown while lazy chunk loads)
const PageLoader = () => (
  <div className="flex items-center justify-center h-screen">
    <div className="h-8 w-8 animate-spin rounded-full border-4 border-muted border-t-foreground" />
  </div>
);

const App: React.FC = () => {
  return (
    <Router>
      <ThemeProvider>
        <AuthProvider>
          <AppsProvider>
            <ErrorBoundary>
            <Suspense fallback={<PageLoader />}>
              <Routes>
                {/* Landing page — standalone, no layout wrapper */}
                <Route path="/" element={<LandingPage />} />

                {/* Auth routes — centered card layout */}
                <Route element={<AuthLayout />}>
                  <Route path="/login" element={<Login />} />
                  <Route path="/register" element={<Register />} />
                  <Route path="/forgot-password" element={<ForgotPassword />} />
                  <Route path="/reset-password" element={<ResetPassword />} />
                </Route>

                {/* SSO callback — no layout */}
                <Route path="/auth/callback" element={<SSOCallback />} />

                {/* Protected dashboard routes — sidebar layout */}
                <Route element={<ProtectedRoute><DashboardLayout /></ProtectedRoute>}>
                  <Route path="/apps" element={<AppsList />} />
                  <Route path="/apps/:id" element={<AppDetail />} />
                  <Route path="/tenants" element={<TenantsList />} />
                  <Route path="/tenants/:id" element={<TenantDetail />} />
                  <Route path="/apps/:id/templates/library" element={<TemplateLibrary />} />
                  <Route path="/dashboard" element={<Dashboard />} />
                  <Route path="/workflows" element={<WorkflowsList />} />
                  <Route path="/workflows/new" element={<WorkflowBuilder />} />
                  <Route path="/workflows/:id" element={<WorkflowBuilder />} />
                  <Route path="/workflows/executions" element={<WorkflowExecutions />} />
                  <Route path="/digest-rules" element={<DigestRulesList />} />
                  <Route path="/topics" element={<TopicsList />} />
                  <Route path="/audit" element={<AuditLogsList />} />
                </Route>

                {/* Documentation hub — public, no auth required */}
                <Route element={<DashboardLayout />}>
                  <Route path="/docs" element={<DocsLayout />}>
                    <Route index element={<Navigate to="/docs/getting-started" replace />} />
                    <Route path="api" element={<ApiReferencePage />} />
                    <Route path=":slug" element={<DocsPage />} />
                  </Route>
                </Route>

                {/* Catch-all redirect */}
                <Route path="*" element={<Navigate to="/" replace />} />
              </Routes>
            </Suspense>
          </ErrorBoundary>
          <CommandPalette />
          <Toaster />
          </AppsProvider>
        </AuthProvider>
      </ThemeProvider>
    </Router>
  );
};

export default App;